# app/models/tick.py
"""
Pydantic models for market ticks
Maps from Go's msgpack-encoded Tick struct
"""
from datetime import datetime
from typing import Literal
from pydantic import BaseModel, Field, field_validator


class Tick(BaseModel):
    """
    Normalized market price update
    Matches Go's internal/models/tick.go structure
    """

    source: Literal["KALSHI", "POLYMARKET"] = Field(
        ...,
        description="Data source platform"
    )

    contract_id: str = Field(
        ...,
        min_length=1,
        description="Contract identifier (ticker or token_id)"
    )

    price: float = Field(
        ...,
        ge=0.0,
        le=1.0,
        description="Probability as decimal (0.0 - 1.0)"
    )

    ts_source: int = Field(
        ...,
        description="Exchange timestamp in milliseconds"
    )

    ts_ingest: int = Field(
        ...,
        description="Our ingestion timestamp in milliseconds"
    )

    ts_emit: int | None = Field(
        default=None,
        description="Frontend emit timestamp in milliseconds"
    )

    @field_validator("source", mode="before")
    @classmethod
    def normalize_source(cls, v: str) -> str:
        """Normalize source to uppercase."""
        if isinstance(v, str):
            return v.upper()
        return v

    @field_validator("price")
    @classmethod
    def validate_price(cls, v: float) -> float:
        """Ensure price is valid probability"""
        if not 0.0 <= v <= 1.0:
            raise ValueError(f"price must be between 0.0 and 1.0, got: {v}")
        return v

    @property
    def latency_ingest_ms(self) -> int:
        """Latency from source to ingestion in milliseconds"""
        return self.ts_ingest - self.ts_source

    @property
    def latency_emit_ms(self) -> int | None:
        """Total latency from source to frontend emission in milliseconds"""
        if self.ts_emit is None:
            return None
        return self.ts_emit - self.ts_source

    @property
    def source_time(self) -> datetime:
        """Convert source timestamp to datetime"""
        return datetime.fromtimestamp(self.ts_source / 1000.0)

    @property
    def ingest_time(self) -> datetime:
        """Convert ingest timestamp to datetime"""
        return datetime.fromtimestamp(self.ts_ingest / 1000.0)

    model_config = {
        "json_schema_extra": {
            "examples": [
                {
                    "source": "KALSHI",
                    "contract_id": "FED-25MAR-T4.75",
                    "price": 0.35,
                    "ts_source": 1706000000000,
                    "ts_ingest": 1706000000050,
                    "ts_emit": 1706000000100
                }
            ]
        }
    }


class TickBatch(BaseModel):
    """Batch of ticks for bulk operations"""

    ticks: list[Tick] = Field(
        ...,
        min_length=1,
        description="List of tick updates"
    )

    batch_timestamp: datetime = Field(
        default_factory=datetime.now,
        description="Batch processing timestamp"
    )


class LatencyStats(BaseModel):
    """Latency statistics for a data source"""

    source: str = Field(..., description="Data source name")
    avg_latency_ms: float = Field(..., description="Average latency in milliseconds")
    p50_latency_ms: float = Field(..., description="Median latency")
    p95_latency_ms: float = Field(..., description="95th percentile latency")
    p99_latency_ms: float = Field(..., description="99th percentile latency")
    max_latency_ms: float = Field(..., description="Maximum latency")
    sample_count: int = Field(..., description="Number of samples")
    time_window_seconds: int = Field(
        default=60,
        description="Time window for statistics"
    )
