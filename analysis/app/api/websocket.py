# app/api/websocket.py
"""
WebSocket handlers for real-time spread updates
"""
import asyncio
import logging
import json
from datetime import datetime
from typing import Set

from fastapi import APIRouter, WebSocket, WebSocketDisconnect
from redis import asyncio as aioredis
import msgpack
from websockets.exceptions import ConnectionClosed

from app.config import settings
from app.models.tick import Tick

# Import ClientDisconnected from uvicorn to handle specific disconnect errors
try:
    from uvicorn.protocols.utils import ClientDisconnected
except ImportError:
    # Fallback if uvicorn internal structure changes
    class ClientDisconnected(Exception):
        pass

logger = logging.getLogger(__name__)

router = APIRouter()


class ConnectionManager:
    """
    Manages WebSocket connections

    Handles broadcasting spread updates to all connected clients
    """

    def __init__(self):
        self.active_connections: Set[WebSocket] = set()
        self.connection_count = 0

    async def connect(self, websocket: WebSocket) -> bool:
        """
        Accept a new WebSocket connection

        Returns:
            True if connection accepted, False if max connections reached
        """
        if len(self.active_connections) >= settings.ws_max_connections:
            logger.warning(f"Max connections reached: {settings.ws_max_connections}")
            return False

        await websocket.accept()
        self.active_connections.add(websocket)
        self.connection_count += 1
        logger.info(f"WebSocket connected (total: {len(self.active_connections)})")
        return True

    def disconnect(self, websocket: WebSocket):
        """Remove a WebSocket connection"""
        if websocket in self.active_connections:
            self.active_connections.remove(websocket)
            logger.info(f"WebSocket disconnected (total: {len(self.active_connections)})")

    async def broadcast(self, message: dict):
        """
        Broadcast message to all connected clients

        Removes dead connections automatically
        """
        dead_connections = set()

        for connection in self.active_connections:
            try:
                await connection.send_json(message)
            except Exception as e:
                logger.warning(f"Failed to send to client: {e}")
                dead_connections.add(connection)

        # Clean up dead connections
        for connection in dead_connections:
            self.disconnect(connection)

    def get_stats(self) -> dict:
        """Get connection statistics"""
        return {
            "active_connections": len(self.active_connections),
            "total_connections": self.connection_count
        }


# Global connection manager
manager = ConnectionManager()


@router.websocket("/ws/spreads")
async def websocket_spreads(websocket: WebSocket):
    """
    WebSocket endpoint for real-time spread updates

    Clients receive spread updates whenever prices change on any platform.

    Message format:
    {
        "type": "spread_update",
        "timestamp": "2025-01-26T10:30:00Z",
        "spreads": [...]
    }
    """
    from app.main import get_app_state

    # Accept connection
    if not await manager.connect(websocket):
        await websocket.close(code=1008, reason="Max connections reached")
        return

    state = get_app_state()

    try:
        # Send initial spreads
        if state.spread_calculator and state.market_pairs:
            initial_spreads = state.spread_calculator.calculate_all_spreads(
                state.market_pairs
            )
            await websocket.send_json({
                "type": "initial_spreads",
                "timestamp": datetime.now().isoformat(),
                "spreads": [s.model_dump(mode='json') for s in initial_spreads]
            })

        # Subscribe to Redis Pub/Sub for real-time updates
        if state.redis_client:
            pubsub = state.redis_client.pubsub()
            await pubsub.psubscribe("tick:*")

            try:
                # Listen for tick updates
                async for message in pubsub.listen():
                    if message["type"] == "pmessage":
                        try:
                            # Decode tick
                            tick_data = msgpack.unpackb(message["data"], raw=False)
                            tick = Tick(**tick_data)

                            # Add emit timestamp
                            tick.ts_emit = int(datetime.now().timestamp() * 1000)

                            # Calculate updated spreads
                            spreads = state.spread_calculator.calculate_all_spreads(
                                state.market_pairs
                            )

                            # Send to client
                            await websocket.send_json({
                                "type": "spread_update",
                                "timestamp": datetime.now().isoformat(),
                                "trigger_tick": {
                                    "source": tick.source,
                                    "contract_id": tick.contract_id,
                                    "price": tick.price,
                                    "latency_ms": tick.latency_emit_ms
                                },
                                "spreads": [s.model_dump(mode='json') for s in spreads]
                            })

                        except (ConnectionClosed, ClientDisconnected):
                            # Re-raise disconnects to be handled by the outer block
                            raise
                        except Exception as e:
                            logger.error(f"Error processing tick: {e}", exc_info=True)
                            # Don't disconnect on processing errors

            except asyncio.CancelledError:
                logger.info("WebSocket task cancelled")
            finally:
                await pubsub.unsubscribe()
        else:
            # No Redis connection, just keep connection alive with heartbeat
            logger.warning("Redis not connected, WebSocket will only send heartbeats")
            while True:
                await asyncio.sleep(settings.ws_heartbeat_interval)
                await websocket.send_json({
                    "type": "heartbeat",
                    "timestamp": datetime.now().isoformat()
                })

    except (WebSocketDisconnect, ConnectionClosed, ClientDisconnected):
        logger.info("Client disconnected normally")
    except Exception as e:
        logger.error(f"WebSocket error: {e}", exc_info=True)
    finally:
        manager.disconnect(websocket)


@router.websocket("/ws/ticks")
async def websocket_ticks(websocket: WebSocket):
    """
    WebSocket endpoint for raw tick stream

    Clients receive every tick update from all platforms.

    Message format:
    {
        "type": "tick",
        "tick": {...}
    }
    """
    from app.main import get_app_state

    # Accept connection
    if not await manager.connect(websocket):
        await websocket.close(code=1008, reason="Max connections reached")
        return

    state = get_app_state()

    try:
        if not state.redis_client:
            logger.warning("Redis not connected")
            await websocket.close(code=1011, reason="Redis not available")
            return

        # Subscribe to all tick channels
        pubsub = state.redis_client.pubsub()
        await pubsub.psubscribe("tick:*")

        try:
            async for message in pubsub.listen():
                if message["type"] == "pmessage":
                    try:
                        # Decode and forward tick
                        tick_data = msgpack.unpackb(message["data"], raw=False)
                        tick = Tick(**tick_data)

                        # Add emit timestamp
                        tick.ts_emit = int(datetime.now().timestamp() * 1000)

                        await websocket.send_json({
                            "type": "tick",
                            "timestamp": datetime.now().isoformat(),
                            "tick": tick.model_dump(mode='json')
                        })

                    except (ConnectionClosed, ClientDisconnected):
                        # Re-raise disconnects to be handled by the outer block
                        raise
                    except Exception as e:
                        logger.error(f"Error forwarding tick: {e}", exc_info=True)

        except asyncio.CancelledError:
            logger.info("Tick WebSocket task cancelled")
        finally:
            await pubsub.unsubscribe()

    except (WebSocketDisconnect, ConnectionClosed, ClientDisconnected):
        logger.info("Client disconnected from tick stream")
    except Exception as e:
        logger.error(f"Tick WebSocket error: {e}", exc_info=True)
    finally:
        manager.disconnect(websocket)


@router.get("/ws/stats")
async def websocket_stats():
    """
    Get WebSocket connection statistics

    Returns info about active connections
    """
    return manager.get_stats()