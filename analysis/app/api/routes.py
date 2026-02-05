# app/api/routes.py
"""
REST API Routes for tick data
"""
import logging
from datetime import datetime, timezone

from fastapi import APIRouter, HTTPException, Query
import msgpack

from app.models.tick import Tick
from app.config import settings

logger = logging.getLogger(__name__)

router = APIRouter()


@router.get("/ticks")
async def get_ticks(
    limit: int = Query(default=100, ge=1, le=1000, description="Number of ticks to return"),
    source: str = Query(default=None, description="Filter by source (KALSHI or POLYMARKET)")
):
    """
    Get recent raw ticks from Redis Stream.

    Args:
        limit: Maximum number of ticks to return
        source: Optional filter by source platform
    """
    from app.main import get_app_state

    state = get_app_state()

    if not state.redis_client:
        raise HTTPException(status_code=503, detail="Redis not connected")

    try:
        messages = await state.redis_client.xrevrange(
            settings.redis.stream_name,
            max="+",
            min="-",
            count=limit * 2 if source else limit  # Fetch extra if filtering
        )
    except Exception as e:
        logger.error(f"Error reading ticks from Redis: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to fetch ticks")

    ticks = []
    for _, message_data in messages:
        if len(ticks) >= limit:
            break

        data_bytes = message_data.get(b"data") or message_data.get("data")
        if not data_bytes:
            continue
        if isinstance(data_bytes, str):
            data_bytes = data_bytes.encode()
        try:
            tick_dict = msgpack.unpackb(data_bytes, raw=False)
            tick = Tick(**tick_dict)
        except Exception as e:
            logger.warning(f"Skipping invalid tick data: {e}")
            continue

        # Filter by source if specified
        if source and tick.source.upper() != source.upper():
            continue

        timestamp = tick.source_time.replace(tzinfo=timezone.utc).isoformat().replace("+00:00", "Z")
        ticks.append({
            "source": tick.source,
            "contract_id": tick.contract_id,
            "price": tick.price,
            "timestamp": timestamp,
            "latency_ms": tick.latency_ingest_ms
        })

    return ticks


@router.get("/stats/consumer")
async def get_consumer_stats():
    """
    Get Redis Stream consumer statistics
    """
    from app.main import get_app_state

    state = get_app_state()

    if not state.consumer:
        return {"error": "Consumer not initialized"}

    try:
        stats = await state.consumer.get_stats()
        pending_count = await state.consumer.get_pending_count()
        stats["pending_messages"] = pending_count
        return stats
    except Exception as e:
        logger.error(f"Error getting consumer stats: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to get consumer stats")


@router.get("/stats/stream")
async def get_stream_stats():
    """
    Get Redis Stream statistics
    """
    from app.main import get_app_state

    state = get_app_state()

    if not state.redis_client:
        raise HTTPException(status_code=503, detail="Redis not connected")

    try:
        info = await state.redis_client.xinfo_stream(settings.redis.stream_name)
        return {
            "length": info.get("length", 0),
            "first_entry": info.get("first-entry"),
            "last_entry": info.get("last-entry"),
            "groups": info.get("groups", 0)
        }
    except Exception as e:
        logger.error(f"Error getting stream stats: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to get stream stats")


@router.post("/debug/tick")
async def debug_post_tick(
    source: str,
    contract_id: str,
    price: float
):
    """
    Debug endpoint to manually post a tick.
    Use this for testing without running the ingestor.
    """
    from app.main import get_app_state

    state = get_app_state()

    if not state.redis_client:
        raise HTTPException(status_code=503, detail="Redis not connected")

    source = source.upper()
    if source not in ["KALSHI", "POLYMARKET"]:
        raise HTTPException(status_code=400, detail="Invalid source (must be KALSHI or POLYMARKET)")

    if not 0.0 <= price <= 1.0:
        raise HTTPException(status_code=400, detail="Price must be between 0.0 and 1.0")

    try:
        timestamp_ms = int(datetime.now(tz=timezone.utc).timestamp() * 1000)
        tick = Tick(
            source=source,
            contract_id=contract_id,
            price=price,
            ts_source=timestamp_ms,
            ts_ingest=timestamp_ms
        )
        data = msgpack.packb(tick.model_dump(), use_bin_type=True)
        await state.redis_client.xadd(
            settings.redis.stream_name,
            {"data": data},
            maxlen=10000,
            approximate=True
        )
        await state.redis_client.publish(f"tick:{contract_id}", data)
        return {
            "success": True,
            "source": source,
            "contract_id": contract_id,
            "price": price,
            "timestamp": datetime.now(tz=timezone.utc).isoformat().replace("+00:00", "Z")
        }
    except Exception as e:
        logger.error(f"Error posting tick: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))
