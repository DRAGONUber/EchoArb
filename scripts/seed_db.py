#!/usr/bin/env python3
"""
scripts/seed_db.py
Database seeding script for EchoArb
Creates tables and optionally seeds with sample data
"""
import asyncio
import sys
from pathlib import Path

# Add parent directory to path
sys.path.insert(0, str(Path(__file__).parent.parent / "analysis"))

from sqlalchemy import create_engine, text
from sqlalchemy.ext.asyncio import create_async_engine, AsyncSession
from sqlalchemy.orm import sessionmaker
from datetime import datetime, timedelta
import random

from app.config import settings
from app.database.models import Base, TickModel, SpreadModel, AlertModel, MarketMetadata, SourceEnum


async def create_tables():
    """Create all database tables"""
    print("Creating database tables...")

    # Use sync engine for table creation
    engine = create_engine(settings.database.sync_url, echo=True)

    try:
        Base.metadata.create_all(engine)
        print("✓ Tables created successfully")
    except Exception as e:
        print(f"✗ Error creating tables: {e}")
        raise
    finally:
        engine.dispose()


async def create_hypertables():
    """Convert tables to TimescaleDB hypertables"""
    print("\nCreating TimescaleDB hypertables...")

    engine = create_async_engine(settings.database.url, echo=False)

    async with engine.begin() as conn:
        try:
            # Create hypertable for ticks
            await conn.execute(
                text("SELECT create_hypertable('ticks', 'timestamp', if_not_exists => TRUE);")
            )
            print("✓ Created hypertable: ticks")
        except Exception as e:
            print(f"  Note: {e}")

        try:
            # Create hypertable for spreads
            await conn.execute(
                text("SELECT create_hypertable('spreads', 'timestamp', if_not_exists => TRUE);")
            )
            print("✓ Created hypertable: spreads")
        except Exception as e:
            print(f"  Note: {e}")

    await engine.dispose()


async def seed_sample_data():
    """Seed database with sample data"""
    print("\nSeeding sample data...")

    engine = create_async_engine(settings.database.url, echo=False)
    async_session = sessionmaker(engine, class_=AsyncSession, expire_on_commit=False)

    async with async_session() as session:
        try:
            # Generate sample ticks (last 24 hours)
            print("  Generating sample ticks...")
            base_time = datetime.utcnow() - timedelta(hours=24)

            contracts = [
                (SourceEnum.KALSHI, "FED-25MAR-T4.75"),
                (SourceEnum.KALSHI, "FED-25MAR-T5.00"),
                (SourceEnum.POLYMARKET, "0x1234567890abcdef"),
            ]

            tick_count = 0
            for hours_ago in range(24, 0, -1):
                tick_time = base_time + timedelta(hours=hours_ago)
                ts_ms = int(tick_time.timestamp() * 1000)

                for source, contract_id in contracts:
                    # Generate realistic price movement
                    base_price = 0.35 if "T4.75" in contract_id else 0.20
                    price = base_price + random.uniform(-0.05, 0.05)
                    price = max(0.01, min(0.99, price))  # Clamp to valid range

                    tick = TickModel(
                        timestamp=tick_time,
                        source=source,
                        contract_id=contract_id,
                        price=price,
                        ts_source=ts_ms,
                        ts_ingest=ts_ms + random.randint(10, 100),
                        latency_ingest_ms=random.randint(10, 100)
                    )
                    session.add(tick)
                    tick_count += 1

            print(f"  ✓ Generated {tick_count} sample ticks")

            # Generate sample spreads
            print("  Generating sample spreads...")
            spread_count = 0
            for hours_ago in range(24, 0, -1):
                spread_time = base_time + timedelta(hours=hours_ago)

                kalshi_prob = 0.55 + random.uniform(-0.1, 0.1)
                poly_prob = 0.58 + random.uniform(-0.1, 0.1)
                spread = SpreadModel(
                    timestamp=spread_time,
                    pair_id="fed-rate-march-2025",
                    description="Federal Reserve interest rate decision March 2025",
                    kalshi_prob=kalshi_prob,
                    poly_prob=poly_prob,
                    kalshi_poly_spread=abs(kalshi_prob - poly_prob),
                    max_spread=abs(kalshi_prob - poly_prob),
                    max_spread_pair="KALSHI-POLY",
                    data_completeness=1.0
                )
                session.add(spread)
                spread_count += 1

            print(f"  ✓ Generated {spread_count} sample spreads")

            # Generate sample alerts
            print("  Generating sample alerts...")
            alert = AlertModel(
                created_at=datetime.utcnow() - timedelta(hours=2),
                pair_id="fed-rate-march-2025",
                spread=0.08,
                threshold=0.05,
                severity="medium",
                spread_data={
                    "kalshi_prob": 0.60,
                    "poly_prob": 0.52,
                    "max_spread": 0.08
                }
            )
            session.add(alert)
            print("  ✓ Generated 1 sample alert")

            # Add market metadata
            print("  Adding market metadata...")
            metadata_items = [
                MarketMetadata(
                    source=SourceEnum.KALSHI,
                    contract_id="FED-25MAR-T4.75",
                    title="Fed Rate 4.75-5.00% March 2025",
                    description="Will the Federal Reserve set rates between 4.75% and 5.00% in March 2025?",
                    close_time=datetime(2025, 3, 19, 14, 0, 0),
                    volume_24h=125000.0,
                    metadata={"category": "economics", "tags": ["fed", "interest-rates"]}
                ),
                MarketMetadata(
                    source=SourceEnum.POLYMARKET,
                    contract_id="0x1234567890abcdef",
                    title="Fed Rate > 4.75% March 2025",
                    description="Will the Federal Reserve set rates above 4.75% in March 2025?",
                    close_time=datetime(2025, 3, 19, 14, 0, 0),
                    volume_24h=89000.0,
                    metadata={"category": "economics", "liquidity": "high"}
                ),
            ]

            for item in metadata_items:
                session.add(item)

            print(f"  ✓ Added {len(metadata_items)} market metadata entries")

            # Commit all changes
            await session.commit()
            print("\n✓ Sample data seeded successfully")

        except Exception as e:
            print(f"\n✗ Error seeding data: {e}")
            await session.rollback()
            raise

    await engine.dispose()


async def main():
    """Main seeding function"""
    print("=" * 60)
    print("EchoArb Database Seeding Script")
    print("=" * 60)
    print(f"\nDatabase: {settings.database.url}")
    print("")

    try:
        # Create tables
        await create_tables()

        # Create hypertables (TimescaleDB)
        await create_hypertables()

        # Seed sample data
        response = input("\nDo you want to seed sample data? (y/n): ")
        if response.lower() == 'y':
            await seed_sample_data()
        else:
            print("Skipping sample data seeding")

        print("\n" + "=" * 60)
        print("Database setup complete!")
        print("=" * 60)

    except Exception as e:
        print(f"\nError: {e}")
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(main())
