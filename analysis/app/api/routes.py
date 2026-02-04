# app/api/routes.py
"""
REST API Routes
"""
import json
import logging
from typing import List
from datetime import datetime, timezone

from fastapi import APIRouter, HTTPException, Query
import msgpack

from app.models.spread import SpreadResult, Alert
from app.models.tick import Tick
from app.config import settings

logger = logging.getLogger(__name__)

router = APIRouter()


@router.get("/spreads", response_model=List[SpreadResult])
async def get_spreads():
    """
    Get current spread calculations for all market pairs

    Returns list of spreads with probabilities from each platform
    """
    from app.main import get_app_state

    state = get_app_state()

    if not state.spread_calculator or not state.market_pairs:
        return []

    try:
        results = state.spread_calculator.calculate_all_spreads(state.market_pairs)
        return results
    except Exception as e:
        logger.error(f"Error calculating spreads: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to calculate spreads")


@router.get("/spreads/{pair_id}", response_model=SpreadResult)
async def get_spread_by_id(pair_id: str):
    """
    Get spread calculation for a specific market pair

    Args:
        pair_id: Market pair identifier
    """
    from app.main import get_app_state

    state = get_app_state()

    if not state.spread_calculator or not state.market_pairs:
        raise HTTPException(status_code=404, detail="Market pair not found")

    # Find the config for this pair
    config = next((p for p in state.market_pairs if p.id == pair_id), None)
    if not config:
        raise HTTPException(status_code=404, detail=f"Market pair '{pair_id}' not found")

    try:
        result = state.spread_calculator.calculate_spread(config)
        if result is None:
            raise HTTPException(
                status_code=503,
                detail="Insufficient data to calculate spread (need at least 2 sources)"
            )
        return result
    except Exception as e:
        logger.error(f"Error calculating spread for {pair_id}: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to calculate spread")


@router.get("/alerts", response_model=List[Alert])
async def get_alerts(
    min_threshold: float = Query(default=0.05, ge=0.0, le=1.0, description="Minimum spread threshold")
):
    """
    Get active arbitrage alerts

    Returns spreads that exceed the alert threshold
    """
    from app.main import get_app_state

    state = get_app_state()

    if not state.spread_calculator or not state.market_pairs:
        return []

    try:
        # Get all spreads exceeding threshold
        alert_spreads = state.spread_calculator.get_alerts(state.market_pairs)

        # Convert to Alert objects
        alerts = []
        for spread in alert_spreads:
            alert = Alert.from_spread(spread, threshold=min_threshold)
            if alert:
                alerts.append(alert)

        return alerts
    except Exception as e:
        logger.error(f"Error getting alerts: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to get alerts")


@router.get("/ticks")
async def get_ticks(
    limit: int = Query(default=100, ge=1, le=1000, description="Number of ticks to return")
):
    """
    Get recent raw ticks from Redis Stream.
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
            count=limit
        )
    except Exception as e:
        logger.error(f"Error reading ticks from Redis: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to fetch ticks")

    ticks = []
    for _, message_data in messages:
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

        timestamp = tick.source_time.replace(tzinfo=timezone.utc).isoformat().replace("+00:00", "Z")
        ticks.append({
            "source": tick.source,
            "contract_id": tick.contract_id,
            "price": tick.price,
            "timestamp": timestamp,
            "latency_ms": tick.latency_ingest_ms
        })

    return ticks


@router.get("/pairs")
async def get_market_pairs():
    """
    Get recent raw ticks from Redis Stream.
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
            count=limit
        )
    except Exception as e:
        logger.error(f"Error reading ticks from Redis: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to fetch ticks")

    ticks = []
    for _, message_data in messages:
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

        timestamp = tick.source_time.replace(tzinfo=timezone.utc).isoformat().replace("+00:00", "Z")
        ticks.append({
            "source": tick.source,
            "contract_id": tick.contract_id,
            "price": tick.price,
            "timestamp": timestamp,
            "latency_ms": tick.latency_ingest_ms
        })

    return ticks


@router.get("/pairs")
async def get_market_pairs():
    """
    Legacy endpoint for configured market pairs (deprecated).

    Returns subscription configuration from the config file.
    """
    return await get_subscriptions()


@router.get("/subscriptions")
async def get_subscriptions():
    """
    Get list of configured market subscriptions.
    """
    config_path = Path(settings.market_pairs_path)
    if not config_path.exists():
        return {"subscriptions": [], "count": 0}

    try:
        with open(config_path) as f:
            data = json.load(f)
    except Exception as e:
        logger.error(f"Failed to load subscriptions: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to load subscriptions")

    subscriptions = data.get("subscriptions", [])
    return {"subscriptions": subscriptions, "count": len(subscriptions)}


@router.get("/stats/cache")
async def get_cache_stats():
    """
    Get price cache statistics

    Returns info about cached prices from each platform
    """
    from app.main import get_app_state

    state = get_app_state()

    if not state.spread_calculator:
        return {"error": "Spread calculator not initialized"}

    try:
        stats = state.spread_calculator.get_cache_stats()
        return stats
    except Exception as e:
        logger.error(f"Error getting cache stats: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to get cache stats")


@router.get("/stats/consumer")
async def get_consumer_stats():
    """
    Get Redis Stream consumer statistics

    Returns info about message processing
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


@router.get("/stats/latency")
async def get_latency_stats():
    """
    Get latency statistics for data sources

    Returns latency metrics for each platform
    """
    from app.main import get_app_state

    state = get_app_state()

    if not state.spread_calculator:
        return {"sources": []}

    # This is a placeholder - would need to track latency in spread_calculator
    # For now, return empty structure
    return {
        "sources": [],
        "note": "Latency tracking not yet implemented in spread calculator"
    }


@router.post("/debug/update_price")
async def debug_update_price(
    source: str,
    contract_id: str,
    price: float
):
    """
    Debug endpoint to manually update a price

    Use this for testing without running the ingestor
    """
    from app.main import get_app_state

    state = get_app_state()

    if not state.redis_client:
        raise HTTPException(status_code=503, detail="Redis not connected")

    # Validate inputs
    if source not in ["KALSHI", "POLYMARKET", "MANIFOLD"]:
        raise HTTPException(status_code=400, detail="Invalid source")

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
        logger.error(f"Error updating price: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))
