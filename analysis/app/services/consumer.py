# app/services/consumer.py
"""
Redis Stream Consumer
Reads ticks from Redis Streams for monitoring and acknowledgments
"""
import asyncio
import logging
from typing import Optional
import msgpack
from redis import asyncio as aioredis

from app.config import settings
from app.services.spread_calculator import SpreadCalculator
from app.models.tick import Tick

logger = logging.getLogger(__name__)


class RedisStreamConsumer:
    """
    Consumes market ticks from Redis Stream

    Uses consumer groups for reliable processing with acknowledgments.
    Never crashes - logs errors and continues processing.
    """

    def __init__(
        self,
        redis_client: aioredis.Redis,
        spread_calculator: SpreadCalculator | None = None
    ):
        self.redis = redis_client
        self.spread_calc = spread_calculator
        self.stream_name = settings.redis.stream_name
        self.consumer_group = settings.redis.consumer_group
        self.consumer_name = settings.redis.consumer_name
        self.running = False
        self._task: Optional[asyncio.Task] = None

        # Statistics
        self.messages_processed = 0
        self.messages_failed = 0
        self.last_message_id: Optional[str] = None

    async def start(self):
        """Start consuming from the stream"""
        if self.running:
            logger.warning("Consumer already running")
            return

        logger.info(
            f"Starting Redis Stream consumer: stream={self.stream_name}, "
            f"group={self.consumer_group}, name={self.consumer_name}"
        )

        # Create consumer group if it doesn't exist
        try:
            await self.redis.xgroup_create(
                name=self.stream_name,
                groupname=self.consumer_group,
                id="0",
                mkstream=True
            )
            logger.info(f"Created consumer group: {self.consumer_group}")
        except aioredis.ResponseError as e:
            if "BUSYGROUP" in str(e):
                logger.info(f"Consumer group already exists: {self.consumer_group}")
            else:
                logger.error(f"Failed to create consumer group: {e}")
                raise

        self.running = True
        self._task = asyncio.create_task(self._consume_loop())
        logger.info("Consumer started successfully")

    async def stop(self):
        """Stop consuming from the stream"""
        if not self.running:
            return

        logger.info("Stopping Redis Stream consumer")
        self.running = False

        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass

        logger.info(
            f"Consumer stopped. Stats: processed={self.messages_processed}, "
            f"failed={self.messages_failed}"
        )

    async def _consume_loop(self):
        """Main consumer loop"""
        last_id = ">"  # Only new messages

        while self.running:
            try:
                # Read from stream using consumer group
                # XREADGROUP returns: [[stream_name, [(message_id, {field: value})]]]
                messages = await self.redis.xreadgroup(
                    groupname=self.consumer_group,
                    consumername=self.consumer_name,
                    streams={self.stream_name: last_id},
                    count=settings.consumer_batch_size,
                    block=settings.consumer_block_ms
                )

                if not messages:
                    # No new messages, continue waiting
                    continue

                # Process messages from this batch
                for stream_name, message_list in messages:
                    for message_id, message_data in message_list:
                        await self._process_message(
                            message_id.decode() if isinstance(message_id, bytes) else message_id,
                            message_data
                        )

            except asyncio.CancelledError:
                logger.info("Consumer loop cancelled")
                break
            except Exception as e:
                logger.error(f"Error in consumer loop: {e}", exc_info=True)
                # Don't crash - wait a bit and retry
                await asyncio.sleep(1)

    async def _process_message(self, message_id: str, message_data: dict):
        """
        Process a single message from the stream

        Args:
            message_id: Redis stream message ID
            message_data: Message fields (should contain 'data' field with msgpack bytes)
        """
        try:
            # Extract msgpack data
            # Redis returns bytes with b'' prefix
            data_bytes = message_data.get(b"data") or message_data.get("data")

            if not data_bytes:
                logger.warning(f"Message {message_id} missing 'data' field")
                # Acknowledge anyway to avoid reprocessing
                await self.redis.xack(self.stream_name, self.consumer_group, message_id)
                self.messages_failed += 1
                return

            # Decode msgpack
            try:
                tick_dict = msgpack.unpackb(data_bytes, raw=False)
            except Exception as e:
                logger.error(f"Failed to decode msgpack from message {message_id}: {e}")
                await self.redis.xack(self.stream_name, self.consumer_group, message_id)
                self.messages_failed += 1
                return

            # Validate with Pydantic model
            try:
                tick = Tick(**tick_dict)
            except Exception as e:
                logger.error(f"Invalid tick data in message {message_id}: {e}")
                logger.debug(f"Tick data: {tick_dict}")
                await self.redis.xack(self.stream_name, self.consumer_group, message_id)
                self.messages_failed += 1
                return

            # Update spread calculator
            if self.spread_calc:
                try:
                    self.spread_calc.update_price(
                        source=tick.source,
                        contract_id=tick.contract_id,
                        price=tick.price
                    )
                except Exception as e:
                    logger.error(f"Failed to update spread calculator: {e}", exc_info=True)
                    # Still acknowledge to avoid infinite retries
                    await self.redis.xack(self.stream_name, self.consumer_group, message_id)
                    self.messages_failed += 1
                    return

            # Acknowledge successful processing
            await self.redis.xack(self.stream_name, self.consumer_group, message_id)

            self.messages_processed += 1
            self.last_message_id = message_id

            # Log periodically
            if self.messages_processed % 100 == 0:
                logger.info(
                    f"Consumer stats: processed={self.messages_processed}, "
                    f"failed={self.messages_failed}, "
                    f"last_id={message_id}"
                )

        except Exception as e:
            logger.error(f"Unexpected error processing message {message_id}: {e}", exc_info=True)
            # Try to acknowledge anyway
            try:
                await self.redis.xack(self.stream_name, self.consumer_group, message_id)
            except Exception as ack_error:
                logger.error(f"Failed to acknowledge message: {ack_error}")
            self.messages_failed += 1

    async def get_stats(self) -> dict:
        """Get consumer statistics"""
        return {
            "running": self.running,
            "messages_processed": self.messages_processed,
            "messages_failed": self.messages_failed,
            "last_message_id": self.last_message_id,
            "stream_name": self.stream_name,
            "consumer_group": self.consumer_group,
            "consumer_name": self.consumer_name
        }

    async def get_pending_count(self) -> int:
        """Get number of pending messages for this consumer"""
        try:
            # XPENDING returns info about pending messages
            pending_info = await self.redis.xpending(
                self.stream_name,
                self.consumer_group
            )
            # pending_info is [count, start_id, end_id, consumers]
            if pending_info:
                return pending_info[0]
            return 0
        except Exception as e:
            logger.error(f"Failed to get pending count: {e}")
            return 0
