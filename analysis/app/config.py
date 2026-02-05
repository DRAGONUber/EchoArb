# app/config.py
"""
Configuration management using Pydantic Settings
Loads from environment variables with validation
"""
from typing import List
from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class RedisSettings(BaseSettings):
    """Redis connection settings"""

    host: str = Field(default="localhost", description="Redis host")
    port: int = Field(default=6379, ge=1, le=65535, description="Redis port")
    password: str = Field(default="", description="Redis password")
    db: int = Field(default=0, ge=0, description="Redis database number")
    pool_size: int = Field(default=10, ge=1, description="Connection pool size")
    min_idle_conns: int = Field(default=5, ge=0, description="Minimum idle connections")
    stream_name: str = Field(default="market_ticks", description="Redis Stream name")
    consumer_group: str = Field(default="tick_consumers", description="Consumer group name")
    consumer_name: str = Field(default="worker_1", description="Consumer name")

    model_config = SettingsConfigDict(env_prefix="REDIS_")


class DatabaseSettings(BaseSettings):
    """PostgreSQL/TimescaleDB settings"""

    host: str = Field(default="localhost", description="Database host")
    port: int = Field(default=5432, ge=1, le=65535, description="Database port")
    database: str = Field(default="echoarb", description="Database name")
    user: str = Field(default="postgres", description="Database user")
    password: str = Field(default="", description="Database password")
    pool_size: int = Field(default=20, ge=1, description="Connection pool size")
    max_overflow: int = Field(default=10, ge=0, description="Max overflow connections")
    echo: bool = Field(default=False, description="Echo SQL queries (dev only)")

    model_config = SettingsConfigDict(env_prefix="DB_")

    @property
    def url(self) -> str:
        """Database connection URL"""
        return f"postgresql+asyncpg://{self.user}:{self.password}@{self.host}:{self.port}/{self.database}"

    @property
    def sync_url(self) -> str:
        """Synchronous database URL for migrations"""
        return f"postgresql://{self.user}:{self.password}@{self.host}:{self.port}/{self.database}"


class APISettings(BaseSettings):
    """FastAPI application settings"""

    host: str = Field(default="0.0.0.0", description="API host")
    port: int = Field(default=8000, ge=1, le=65535, description="API port")
    reload: bool = Field(default=False, description="Auto-reload on code changes")
    workers: int = Field(default=1, ge=1, description="Number of worker processes")
    cors_origins: List[str] = Field(
        default=["http://localhost:3000", "http://localhost:3001"],
        description="Allowed CORS origins"
    )

    model_config = SettingsConfigDict(env_prefix="API_")


class MonitoringSettings(BaseSettings):
    """Prometheus and metrics settings"""

    enabled: bool = Field(default=True, description="Enable Prometheus metrics")
    port: int = Field(default=9091, ge=1, le=65535, description="Metrics endpoint port")
    path: str = Field(default="/metrics", description="Metrics endpoint path")

    model_config = SettingsConfigDict(env_prefix="METRICS_")


class Settings(BaseSettings):
    """Main application settings"""

    # Environment
    environment: str = Field(default="development", description="Environment: development, staging, production")
    log_level: str = Field(default="INFO", description="Logging level")
    debug: bool = Field(default=False, description="Debug mode")

    # WebSocket settings
    ws_heartbeat_interval: int = Field(
        default=30,
        ge=5,
        description="WebSocket heartbeat interval in seconds"
    )
    ws_max_connections: int = Field(
        default=1000,
        ge=1,
        description="Maximum concurrent WebSocket connections"
    )

    # Redis consumer settings
    consumer_batch_size: int = Field(
        default=100,
        ge=1,
        description="Number of messages to read from stream per batch"
    )
    consumer_block_ms: int = Field(
        default=5000,
        ge=100,
        description="Block time in milliseconds when reading from stream"
    )

    # Sub-settings
    redis: RedisSettings = Field(default_factory=RedisSettings)
    database: DatabaseSettings = Field(default_factory=DatabaseSettings)
    api: APISettings = Field(default_factory=APISettings)
    monitoring: MonitoringSettings = Field(default_factory=MonitoringSettings)

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="ignore"
    )

    @field_validator("log_level")
    @classmethod
    def validate_log_level(cls, v: str) -> str:
        """Validate log level"""
        allowed = ["DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"]
        v_upper = v.upper()
        if v_upper not in allowed:
            raise ValueError(f"log_level must be one of {allowed}, got: {v}")
        return v_upper

    @field_validator("environment")
    @classmethod
    def validate_environment(cls, v: str) -> str:
        """Validate environment"""
        allowed = ["development", "staging", "production"]
        v_lower = v.lower()
        if v_lower not in allowed:
            raise ValueError(f"environment must be one of {allowed}, got: {v}")
        return v_lower


# Global settings instance
settings = Settings()


def get_settings() -> Settings:
    """Get settings instance (for FastAPI dependency injection)"""
    return settings
