# app/main.py
"""
EchoArb Analysis API - FastAPI Application
Real-time prediction market arbitrage scanner
"""
import logging
import json
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from prometheus_client import make_asgi_app
from redis import asyncio as aioredis

from app.config import settings
from app.services.spread_calculator import SpreadCalculator, MarketPairConfig
from app.services.consumer import RedisStreamConsumer
from app.services.transformer import TransformStrategy
from app.api import routes, websocket

# Configure logging
logging.basicConfig(
    level=settings.log_level,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)


class AppState:
    """Global application state"""

    def __init__(self):
        self.redis_client: aioredis.Redis | None = None
        self.spread_calculator: SpreadCalculator | None = None
        self.consumer: RedisStreamConsumer | None = None
        self.market_pairs: list[MarketPairConfig] = []


# Global state instance
app_state = AppState()


def load_market_pairs() -> list[MarketPairConfig]:
    """
    Load market pairs configuration from JSON file

    Returns:
        List of MarketPairConfig objects
    """
    config_path = Path(settings.market_pairs_path)

    if not config_path.exists():
        logger.warning(f"Market pairs config not found at {config_path}")
        # Return example configuration for development
        if settings.environment == "development":
            logger.info("Using default development market pair")
            return [
                MarketPairConfig(
                    id="fed-rate-march-2025",
                    description="Federal Reserve interest rate decision March 2025",
                    kalshi_tickers=["FED-25MAR-T4.75", "FED-25MAR-T5.00"],
                    kalshi_transform=TransformStrategy.SUM,
                    poly_token_id="0x1234567890abcdef",
                    manifold_slug="will-the-fed-cut-rates-in-march",
                    alert_threshold=0.05
                )
            ]
        return []

    try:
        with open(config_path) as f:
            data = json.load(f)

        pairs = []
        for pair_data in data.get("pairs", []):
            # Parse transform strategies
            kalshi_transform = TransformStrategy(
                pair_data.get("kalshi_transform", "identity")
            )
            poly_transform = TransformStrategy(
                pair_data.get("poly_transform", "identity")
            )
            manifold_transform = TransformStrategy(
                pair_data.get("manifold_transform", "identity")
            )

            pair = MarketPairConfig(
                id=pair_data["id"],
                description=pair_data["description"],
                kalshi_tickers=pair_data.get("kalshi_tickers", []),
                kalshi_transform=kalshi_transform,
                kalshi_threshold=pair_data.get("kalshi_threshold"),
                poly_token_id=pair_data.get("poly_token_id"),
                poly_transform=poly_transform,
                manifold_slug=pair_data.get("manifold_slug"),
                manifold_transform=manifold_transform,
                alert_threshold=pair_data.get("alert_threshold", settings.alert_threshold_default)
            )
            pairs.append(pair)

        logger.info(f"Loaded {len(pairs)} market pairs from {config_path}")
        return pairs

    except Exception as e:
        logger.error(f"Failed to load market pairs: {e}", exc_info=True)
        return []


@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    Application lifespan manager
    Handles startup and shutdown events
    """
    # Startup
    logger.info(f"Starting EchoArb Analysis API (env={settings.environment})")

    try:
        # Connect to Redis
        app_state.redis_client = aioredis.Redis(
            host=settings.redis.host,
            port=settings.redis.port,
            password=settings.redis.password or None,
            db=settings.redis.db,
            decode_responses=False  # We handle msgpack decoding
        )
        await app_state.redis_client.ping()
        logger.info(f"Connected to Redis at {settings.redis.host}:{settings.redis.port}")

        # Raw tick mode: no spread calculator or market pair matching
        app_state.spread_calculator = None
        app_state.market_pairs = []
        logger.info("Running in raw tick mode (spread calculator disabled)")

        # Start Redis Stream consumer
        app_state.consumer = RedisStreamConsumer(
            redis_client=app_state.redis_client,
            spread_calculator=app_state.spread_calculator
        )
        await app_state.consumer.start()

        logger.info("Application startup complete")

    except Exception as e:
        logger.error(f"Startup failed: {e}", exc_info=True)
        raise

    yield

    # Shutdown
    logger.info("Shutting down EchoArb Analysis API")

    try:
        # Stop consumer
        if app_state.consumer:
            await app_state.consumer.stop()

        # Close Redis connection
        if app_state.redis_client:
            await app_state.redis_client.close()

        logger.info("Shutdown complete")

    except Exception as e:
        logger.error(f"Shutdown error: {e}", exc_info=True)


# Create FastAPI application
app = FastAPI(
    title="EchoArb Analysis API",
    description="Real-time prediction market arbitrage scanner",
    version="1.0.0",
    lifespan=lifespan,
    debug=settings.debug
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.api.cors_origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include routers
app.include_router(routes.router, prefix="/api/v1", tags=["api"])
app.include_router(websocket.router, tags=["websocket"])

# Prometheus metrics endpoint
if settings.monitoring.enabled:
    metrics_app = make_asgi_app()
    app.mount("/metrics", metrics_app)


@app.get("/health")
async def health_check():
    """Health check endpoint"""
    redis_ok = False
    try:
        if app_state.redis_client:
            await app_state.redis_client.ping()
            redis_ok = True
    except Exception:
        pass

    return {
        "status": "healthy" if redis_ok else "degraded",
        "environment": settings.environment,
        "redis_connected": redis_ok,
        "market_pairs": len(app_state.market_pairs)
    }


@app.get("/")
async def root():
    """Root endpoint"""
    return {
        "service": "EchoArb Analysis API",
        "version": "1.0.0",
        "environment": settings.environment,
        "docs": "/docs"
    }


# Export app_state for use in other modules
def get_app_state() -> AppState:
    """Get global application state"""
    return app_state
