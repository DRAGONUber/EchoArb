# app/database/models.py
"""
SQLAlchemy ORM models for TimescaleDB
Stores historical tick and spread data for analysis
"""
from datetime import datetime
from sqlalchemy import Column, String, Float, Integer, DateTime, Index, Enum as SQLEnum
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.dialects.postgresql import JSONB
import enum

Base = declarative_base()


class SourceEnum(str, enum.Enum):
    """Data source platforms"""

    KALSHI = "KALSHI"
    POLYMARKET = "POLYMARKET"


class TickModel(Base):
    """
    Historical tick data (time-series)

    This table should be converted to a TimescaleDB hypertable:
    SELECT create_hypertable('ticks', 'timestamp');
    """

    __tablename__ = "ticks"

    id = Column(Integer, primary_key=True, autoincrement=True)
    timestamp = Column(DateTime, nullable=False, index=True)  # Hypertable partition key

    # Tick data
    source = Column(SQLEnum(SourceEnum), nullable=False, index=True)
    contract_id = Column(String(255), nullable=False, index=True)
    price = Column(Float, nullable=False)

    # Timestamps from the tick
    ts_source = Column(Integer, nullable=False)  # Exchange timestamp (ms)
    ts_ingest = Column(Integer, nullable=False)  # Our ingestion time (ms)
    ts_emit = Column(Integer, nullable=True)  # Frontend emit time (ms)

    # Computed fields
    latency_ingest_ms = Column(Integer, nullable=False)  # ts_ingest - ts_source

    __table_args__ = (
        Index("idx_ticks_source_contract", "source", "contract_id"),
        Index("idx_ticks_timestamp_source", "timestamp", "source"),
    )

    def __repr__(self):
        return (
            f"<Tick(source={self.source}, contract={self.contract_id}, "
            f"price={self.price}, ts={self.timestamp})>"
        )


class SpreadModel(Base):
    """
    Historical spread calculations (time-series)

    This table should be converted to a TimescaleDB hypertable:
    SELECT create_hypertable('spreads', 'timestamp');
    """

    __tablename__ = "spreads"

    id = Column(Integer, primary_key=True, autoincrement=True)
    timestamp = Column(DateTime, nullable=False, index=True)  # Hypertable partition key

    # Market pair
    pair_id = Column(String(255), nullable=False, index=True)
    description = Column(String(512), nullable=False)

    # Probabilities
    kalshi_prob = Column(Float, nullable=True)
    poly_prob = Column(Float, nullable=True)

    # Spreads
    kalshi_poly_spread = Column(Float, nullable=True)

    # Maximum spread
    max_spread = Column(Float, nullable=False)
    max_spread_pair = Column(String(50), nullable=False)  # e.g., "KALSHI-POLY"

    # Metadata
    data_completeness = Column(Float, nullable=False)

    __table_args__ = (
        Index("idx_spreads_pair_timestamp", "pair_id", "timestamp"),
        Index("idx_spreads_max_spread", "max_spread"),
    )

    def __repr__(self):
        return (
            f"<Spread(pair={self.pair_id}, max_spread={self.max_spread:.4f}, "
            f"ts={self.timestamp})>"
        )


class AlertModel(Base):
    """
    Alert history

    Stores when arbitrage opportunities exceeded thresholds
    """

    __tablename__ = "alerts"

    id = Column(Integer, primary_key=True, autoincrement=True)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow, index=True)

    # Associated spread
    pair_id = Column(String(255), nullable=False, index=True)
    spread = Column(Float, nullable=False)
    threshold = Column(Float, nullable=False)
    severity = Column(String(20), nullable=False)  # low, medium, high, critical

    # Full spread data as JSON
    spread_data = Column(JSONB, nullable=False)

    # Alert acknowledgment
    acknowledged = Column(DateTime, nullable=True)
    acknowledged_by = Column(String(255), nullable=True)

    __table_args__ = (
        Index("idx_alerts_created_at", "created_at"),
        Index("idx_alerts_pair_severity", "pair_id", "severity"),
        Index("idx_alerts_unacknowledged", "acknowledged"),
    )

    def __repr__(self):
        return (
            f"<Alert(pair={self.pair_id}, spread={self.spread:.4f}, "
            f"severity={self.severity}, ts={self.created_at})>"
        )


class MarketMetadata(Base):
    """
    Market metadata cache

    Stores market information from each platform
    """

    __tablename__ = "market_metadata"

    id = Column(Integer, primary_key=True, autoincrement=True)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    source = Column(SQLEnum(SourceEnum), nullable=False)
    contract_id = Column(String(255), nullable=False, unique=True, index=True)

    # Market info
    title = Column(String(512), nullable=True)
    description = Column(String(2048), nullable=True)
    close_time = Column(DateTime, nullable=True)
    volume_24h = Column(Float, nullable=True)

    # Additional metadata as JSON
    metadata = Column(JSONB, nullable=True)

    __table_args__ = (
        Index("idx_market_metadata_source", "source"),
    )

    def __repr__(self):
        return f"<MarketMetadata(source={self.source}, contract={self.contract_id})>"


# Continuous aggregates for TimescaleDB (example queries to run)
"""
-- Create continuous aggregate for hourly tick statistics
CREATE MATERIALIZED VIEW ticks_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', timestamp) AS bucket,
    source,
    contract_id,
    AVG(price) AS avg_price,
    MIN(price) AS min_price,
    MAX(price) AS max_price,
    AVG(latency_ingest_ms) AS avg_latency_ms,
    COUNT(*) AS tick_count
FROM ticks
GROUP BY bucket, source, contract_id;

-- Create continuous aggregate for hourly spread statistics
CREATE MATERIALIZED VIEW spreads_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', timestamp) AS bucket,
    pair_id,
    AVG(max_spread) AS avg_spread,
    MAX(max_spread) AS max_spread,
    AVG(data_completeness) AS avg_completeness,
    COUNT(*) AS calculation_count
FROM spreads
GROUP BY bucket, pair_id;

-- Refresh policies (optional, auto-refresh every hour)
SELECT add_continuous_aggregate_policy('ticks_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour');

SELECT add_continuous_aggregate_policy('spreads_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour');

-- Retention policy (drop data older than 30 days)
SELECT add_retention_policy('ticks', INTERVAL '30 days');
SELECT add_retention_policy('spreads', INTERVAL '90 days');
"""
