# app/main.py
"""
EchoArb Analysis API - FastAPI Application
Real-time prediction market data streaming
"""
import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from prometheus_client import make_asgi_app
from redis import asyncio as aioredis

from app.config import settings
from app.services.consumer import RedisStreamConsumer
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
        self.consumer: RedisStreamConsumer | None = None


# Global state instance
app_state = AppState()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan manager"""
    logger.info(f"Starting EchoArb Analysis API (env={settings.environment})")

    try:
        # Connect to Redis
        app_state.redis_client = aioredis.Redis(
            host=settings.redis.host,
            port=settings.redis.port,
            password=settings.redis.password or None,
            db=settings.redis.db,
            decode_responses=False
        )
        await app_state.redis_client.ping()
        logger.info(f"Connected to Redis at {settings.redis.host}:{settings.redis.port}")

        # Start Redis Stream consumer
        app_state.consumer = RedisStreamConsumer(redis_client=app_state.redis_client)
        await app_state.consumer.start()

        logger.info("Application startup complete")

    except Exception as e:
        logger.error(f"Startup failed: {e}", exc_info=True)
        raise

    yield

    # Shutdown
    logger.info("Shutting down EchoArb Analysis API")

    try:
        if app_state.consumer:
            await app_state.consumer.stop()

        if app_state.redis_client:
            await app_state.redis_client.close()

        logger.info("Shutdown complete")

    except Exception as e:
        logger.error(f"Shutdown error: {e}", exc_info=True)


# Create FastAPI application
app = FastAPI(
    title="EchoArb Analysis API",
    description="Real-time prediction market data streaming",
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
        "redis_connected": redis_ok
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


def get_app_state() -> AppState:
    """Get global application state"""
    return app_state
