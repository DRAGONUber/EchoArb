# Manifold Markets Integration Removal

EchoArb has been refactored to support only **Kalshi** and **Polymarket**. This document details all changes made to remove Manifold Markets integration.

## Rationale

The decision to remove Manifold Markets was made to:
- Simplify the codebase and reduce maintenance overhead
- Focus on the two most liquid prediction markets
- Reduce data completeness requirements (2 sources instead of 3)
- Eliminate dependencies on Manifold's API

## Changes by Component

### 1. Go Ingestor (ingestor/cmd/ingestor/main.go)

**Removed:**
- Manifold connector initialization
- Manifold goroutine in main loop

**Before:**
```go
go kalshiConn.Start(ctx)
go polyConn.Start(ctx)
go manifoldConn.Start(ctx)
```

**After:**
```go
go kalshiConn.Start(ctx)
go polyConn.Start(ctx)
```

### 2. Connector Code (ingestor/internal/connectors/)

**Deleted Files:**
- `manifold.go` (WebSocket connector for Manifold Markets)

### 3. Configuration (ingestor/internal/config/config.go)

**Removed Fields:**
- `ManifoldAPIURL` from Config struct
- Default Manifold endpoint from configuration

**Removed from MarketPair:**
- `Manifold *ManifoldMarket` field

### 4. Environment Variables (docker-compose.yml)

**Removed from ingestor service:**
```yaml
- MANIFOLD_API_URL=${MANIFOLD_API_URL:-https://api.manifold.markets/v0}
```

### 5. Python Spread Calculator (analysis/app/services/spread_calculator.py)

**Modified SpreadCalculator.price_cache:**

**Before:**
```python
self.price_cache: Dict[str, Dict[str, float]] = {
    "KALSHI": {},
    "POLYMARKET": {},
    "MANIFOLD": {}
}
```

**After:**
```python
self.price_cache: Dict[str, Dict[str, float]] = {
    "KALSHI": {},
    "POLYMARKET": {}
}
```

**Modified calculate_spread():**

**Data Completeness:**
- Before: `available_sources / 3.0` (3 platforms)
- After: `available_sources / 2.0` (2 platforms)
- Now returns `None` if fewer than 2 sources available

**Removed Spreads:**
- `KALSHI-MANIFOLD`
- `POLY-MANIFOLD`

**Retained Spread:**
- `KALSHI-POLY` (only spread calculated)

**Removed Methods:**
- `_get_manifold_probability()` and related transform logic

**SpreadResult Changes:**
```python
# manifold_prob always set to None
manifold_prob=None
```

### 6. Market Pairs Configuration (config/market_pairs.json)

**Removed Fields:**
- `manifold_slug`
- `manifold_transform`

**New Format:**
```json
{
  "id": "market-id",
  "description": "Market description",
  "kalshi_tickers": ["TICKER-1"],
  "kalshi_transform": "identity",
  "poly_token_id": "0x...",
  "poly_transform": "identity",
  "alert_threshold": 0.05
}
```

### 7. Frontend Components

**page.tsx (frontend/src/app/page.tsx):**
- Removed Manifold from latency tracking
- Changed latency state initialization from 3 sources to 2

**MarketPairList.tsx (frontend/src/components/MarketPairList.tsx):**
- Changed grid from `grid-cols-3` to `grid-cols-2`
- Updated data completeness logic to check for 2 sources instead of 3
- Removed Manifold probability display column

**SpreadChart.tsx (frontend/src/components/SpreadChart.tsx):**
- Removed `manifold_prob` from SpreadDataPoint interface (kept for backward compatibility, always null)
- Removed Manifold line from chart
- Changed stats display from 4 columns to 3 columns (Kalshi, Polymarket, Spread)
- Removed Manifold probability rendering

### 8. Data Models

**Tick Model (analysis/app/models/tick.py):**
- Source field still accepts "MANIFOLD" literal for backward compatibility
- No Manifold ticks will be generated

**Spread Model (analysis/app/models/spread.py):**
- `manifold_prob` field retained as optional but always None
- Ensures backward compatibility with existing data

### 9. Documentation

**Updated:**
- `README.md`: Removed all Manifold references
- `REAL_DATA_SETUP.md`: Removed Manifold section from market finding guide
- Created this document (`MANIFOLD_REMOVED.md`)

**Deleted:**
- Section on finding Manifold slugs
- Manifold-specific transform examples
- Manifold API documentation links

## Migration Guide

If you were previously using Manifold Markets, follow these steps:

### Step 1: Update Market Pairs Configuration

Edit `config/market_pairs.json` and remove Manifold fields:

**Before:**
```json
{
  "id": "example",
  "kalshi_tickers": ["TICKER"],
  "kalshi_transform": "identity",
  "poly_token_id": "0x123...",
  "poly_transform": "identity",
  "manifold_slug": "market-slug",
  "manifold_transform": "identity",
  "alert_threshold": 0.05
}
```

**After:**
```json
{
  "id": "example",
  "kalshi_tickers": ["TICKER"],
  "kalshi_transform": "identity",
  "poly_token_id": "0x123...",
  "poly_transform": "identity",
  "alert_threshold": 0.05
}
```

### Step 2: Rebuild Docker Images

```bash
docker-compose down
docker-compose build --no-cache
docker-compose up -d
```

### Step 3: Verify System Operation

```bash
# Check ingestor logs
docker-compose logs -f ingestor
# Should see: Connected to Kalshi, Connected to Polymarket
# Should NOT see: Manifold-related messages

# Check spread calculations
curl http://localhost:8000/api/v1/spreads | jq .
# manifold_prob should be null
# max_spread_pair should be "KALSHI-POLY"
```

## Data Completeness Changes

**Before:**
- Required data from at least 1 platform
- Data completeness: `available_sources / 3.0`
- Could calculate spreads with partial data

**After:**
- Requires data from both Kalshi and Polymarket
- Data completeness: `available_sources / 2.0`
- Returns `None` if either platform is missing data
- More reliable spread calculations

## Benefits of This Change

1. **Simplified Codebase:**
   - Fewer connectors to maintain
   - Reduced configuration complexity
   - Less error handling for third platform

2. **Improved Data Quality:**
   - Both platforms required for spread calculation
   - No partial/incomplete spreads
   - More reliable arbitrage detection

3. **Focused Development:**
   - Concentrate on two most liquid markets
   - Better performance optimization
   - Clearer product scope

4. **Reduced Dependencies:**
   - Fewer external API dependencies
   - Lower risk of third-party API changes breaking system
   - Simpler deployment and monitoring

## Backward Compatibility

The following measures ensure smooth migration:

1. **Data Models:**
   - `manifold_prob` field retained in SpreadResult (always None)
   - Tick model still accepts "MANIFOLD" source (for old data)

2. **Database:**
   - Existing Manifold ticks in TimescaleDB remain accessible
   - Historical spread data with Manifold prices preserved
   - No database migration required

3. **API Responses:**
   - Spread endpoints return same structure
   - `manifold_prob` field present but null
   - Frontend components handle null values gracefully

## Testing Checklist

After removing Manifold, verify:

- [ ] Ingestor starts successfully
- [ ] Kalshi connection established
- [ ] Polymarket connection established
- [ ] No Manifold connection attempts
- [ ] Redis stream receives ticks from both platforms
- [ ] Spread calculator processes ticks correctly
- [ ] Spreads calculated only for KALSHI-POLY
- [ ] Frontend displays two-column grid
- [ ] Charts show only Kalshi and Polymarket lines
- [ ] WebSocket updates work correctly
- [ ] Alerts trigger on KALSHI-POLY spreads
- [ ] No errors in logs related to missing Manifold data

## Rollback Procedure

If you need to restore Manifold integration:

1. Restore deleted file: `ingestor/internal/connectors/manifold.go`
2. Revert changes in `ingestor/cmd/ingestor/main.go`
3. Restore Manifold fields in `config/market_pairs.json`
4. Revert spread calculator changes
5. Restore frontend three-column layouts
6. Rebuild: `docker-compose build --no-cache`

Note: Full rollback requires access to pre-removal commit history.

## Support

For questions or issues related to this change:
- Review this documentation
- Check the main README.md
- Consult REAL_DATA_SETUP.md for current setup instructions
- File issues on GitHub repository

## Summary

EchoArb now operates exclusively with Kalshi and Polymarket. This simplification improves maintainability, data quality, and system reliability while focusing on the two most liquid prediction markets.
