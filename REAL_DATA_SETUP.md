# Setting Up Real Market Data

This guide provides instructions for connecting EchoArb to real prediction market data from Kalshi and Polymarket.

> **Raw Tick Mode**: The current Phase 0 setup streams raw ticks only. Market pairing, transforms, and spread/alert logic are disabled in the analysis service. Use `/api/v1/ticks` and `/ws/spreads` to view live ticks.

## Prerequisites

- Docker and Docker Compose installed
- Active Kalshi account
- Basic understanding of prediction markets
- Terminal/shell access

## Step 1: Obtain Kalshi API Credentials

Kalshi uses RSA-PSS authentication with private keys. The platform generates API credentials that you download and store locally.

### 1.1 Access Kalshi API Settings

Navigate to your Kalshi account API settings:
- URL: https://kalshi.com/account/profile
- Log in with your Kalshi credentials
- Locate the API section

### 1.2 Generate API Key

In the Kalshi dashboard:

1. Click "Create New API Key"
2. Kalshi will generate an RSA keypair
3. You will receive two items:
   - **Private Key** (RSA_PRIVATE_KEY format)
   - **Key ID** (unique identifier, 20-character string)

**Critical**: The private key is displayed only once. Kalshi does not store it. Copy it immediately before closing the dialog.

### 1.3 Save Private Key

Create the keys directory and save the private key:

```bash
mkdir -p keys
```

Copy the private key text from Kalshi and save it to `keys/kalshi_private_key.pem`.

The file should look like this:

```
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA...
(base64 encoded key data)
...
-----END RSA PRIVATE KEY-----
```

Or in PKCS#8 format:

```
-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD...
(base64 encoded key data)
...
-----END PRIVATE KEY-----
```

Set correct file permissions:

```bash
chmod 600 keys/kalshi_private_key.pem
```

### 1.4 Configure Environment Variables

Edit your `.env` file and add:

```bash
KALSHI_API_KEY=<key-id-from-kalshi>
KALSHI_PRIVATE_KEY_PATH=./keys/kalshi_private_key.pem
```

Replace `<key-id-from-kalshi>` with the Key ID provided by Kalshi.

## Step 2: Configure Market Subscriptions

The `config/market_pairs.json` file specifies which markets to subscribe to for raw tick streaming. Transform and alert fields are ignored in raw tick mode.

### 2.1 Understanding Market Identifiers

**Kalshi Market Tickers:**
- Format: `SERIES-DDMMM-TVALUE` (e.g., `FED-25MAR-T4.75`)
- SERIES: Market category (FED, PRES, CPI, etc.)
- DDMMM: Date (25MAR = March 25)
- TVALUE: Threshold value for binary outcome
- Use the market ticker (specific contract), not the event ticker or series

**Polymarket Token IDs:**
- Format: 40-character hexadecimal string starting with `0x`
- Example: `0x1234567890abcdef1234567890abcdef12345678`
- Found in Polymarket's API responses or browser DevTools

### 2.2 Finding Kalshi Market Tickers

Method 1: Browse Kalshi Website
1. Visit https://kalshi.com/markets
2. Navigate to a market of interest
3. The ticker appears in the contract details or URL
4. For ranged markets, note all relevant tickers in the series

Method 2: Use Kalshi API
```bash
# Search for Fed-related markets
curl "https://api.elections.kalshi.com/trade-api/v2/markets?limit=100" | jq '.markets[] | select(.title | contains("Fed"))'
```

### 2.3 Finding Polymarket Token IDs

Method 1: Browser DevTools
1. Visit https://polymarket.com
2. Open browser Developer Tools (F12)
3. Navigate to Network tab
4. Click on a market
5. Inspect API calls to find `token_id` in responses

Method 2: Use Polymarket API
```bash
# Search for markets
curl "https://gamma-api.polymarket.com/markets" | jq '.[] | select(.question | contains("Fed"))'
```

### 2.4 Example Configuration

Edit `config/market_pairs.json`:

```json
{
  "subscriptions": [
    {
      "id": "tick-stream-config",
      "description": "Config for raw tick streaming",
      "kalshi": {
        "ticker": "FED-25MAR-T4.75"
      },
      "polymarket": {
        "token_id": "0x1234567890abcdef1234567890abcdef12345678"
      }
    },
    {
      "id": "presidential-election-2024",
      "description": "2024 US Presidential Election - Republican victory",
      "kalshi": {
        "ticker": "PRES-2024-REP"
      },
      "polymarket": {
        "token_id": "0xabcdef1234567890abcdef1234567890abcdef12"
      }
    }
  ]
}
```

Note: `kalshi_transform`, `poly_transform`, and `alert_threshold` are ignored in raw tick mode.

### 2.5 Transform Strategies Explained (Legacy)

Transform strategies are documented for future phases, but are not applied in raw tick mode.

**identity**: Direct 1:1 mapping
- Use when markets have identical binary outcomes
- Example: "Will X happen?" on both platforms

**sum**: Aggregate multiple Kalshi contracts
- Use when Kalshi has ranged contracts that need to be combined
- Example: Kalshi has "4.75-5.00%" and ">5.00%", but Polymarket has ">4.75%"
- Sum the Kalshi contracts to match Polymarket's binary structure

**inverse**: Flip the probability
- Use when markets represent opposite outcomes
- Example: Kalshi asks "Will X happen?", Polymarket asks "Will X NOT happen?"
- Calculation: 1 - price

### 2.6 When to Use Sum Transform (Legacy)

Kalshi often structures markets as ranges:
- Contract 1: "Rate between 4.75% and 5.00%"
- Contract 2: "Rate above 5.00%"

If Polymarket has a single binary market "Rate above 4.75%", you need to sum Kalshi contracts:

```json
{
  "kalshi_tickers": ["FED-25MAR-T4.75", "FED-25MAR-T5.00"],
  "kalshi_transform": "sum"
}
```

This adds the probabilities: P(4.75-5.00) + P(>5.00) = P(>4.75)

## Step 3: Start Services

### 3.1 Build and Start All Services

```bash
docker-compose up -d
```

This starts:
- Redis (message broker)
- TimescaleDB (time-series database)
- Go ingestor (WebSocket data ingestion)
- Python analysis (raw tick streaming)
- Next.js frontend (dashboard)

### 3.2 View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f ingestor
docker-compose logs -f analysis
```

### 3.3 Monitor Startup

Watch for these log messages:

**Ingestor (Go service):**
```
{"level":"info","msg":"Starting Kalshi connector"}
{"level":"info","msg":"Connected to Kalshi"}
{"level":"info","msg":"Subscribed to Kalshi market: FED-25MAR-T4.75"}
{"level":"info","msg":"Connected to Polymarket"}
```

**Analysis (Python service):**
```
INFO:app.services.consumer:Consumer started successfully
```

## Step 4: Verify Data Flow

### 4.1 Check Ingestor Connection

```bash
docker-compose logs -f ingestor | grep -i "connected"
```

Expected output:
```
Connected to Kalshi
Connected to Polymarket
```

### 4.2 Check Redis Stream

Verify that market data is flowing into Redis:

```bash
# Check stream length (should be > 0)
docker-compose exec redis redis-cli XLEN market_ticks

# View recent messages
docker-compose exec redis redis-cli XRANGE market_ticks - + COUNT 5
```

### 4.3 Test Backend API

```bash
# Health check
curl http://localhost:8000/health
# Expected: {"status":"healthy"}

# View current ticks
curl http://localhost:8000/api/v1/ticks | jq .

# View configured market subscriptions
curl http://localhost:8000/api/v1/pairs | jq .

# Check consumer statistics
curl http://localhost:8000/api/v1/stats/consumer | jq .
```

### 4.4 Access Dashboard

Open http://localhost:3000 in your browser.

Expected indicators:
- WebSocket connection status: "Connected" (green indicator)
- Market subscriptions displayed with real-time prices
- Tick list showing price movements
- Latency metrics for each platform

## Step 5: Troubleshooting

### Issue: Authentication Failure with Kalshi

**Symptoms:**
```
{"level":"error","msg":"failed to get auth headers"}
{"level":"error","msg":"auth error"}
{"level":"fatal","msg":"failed to decode PEM block"}
```

**Solutions:**

1. Verify API Key ID is correct:
```bash
cat .env | grep KALSHI_API_KEY
```

2. Check private key file exists and has correct permissions:
```bash
ls -la keys/kalshi_private_key.pem
# Should show: -rw------- (600 permissions)
```

3. Verify private key is in valid PEM format:
```bash
head -1 keys/kalshi_private_key.pem
# Should show: -----BEGIN PRIVATE KEY----- or -----BEGIN RSA PRIVATE KEY-----
```

4. If permissions are wrong:
```bash
chmod 600 keys/kalshi_private_key.pem
```

5. If PEM format is invalid, you need to regenerate the API key on Kalshi and download a new private key.

### Issue: No Data in Redis

**Symptoms:**
- `XLEN market_ticks` returns 0
- Dashboard shows no data
- No consumer activity in analysis logs

**Solutions:**

1. Check ingestor logs for connection errors:
```bash
docker-compose logs ingestor | grep -i error
```

2. Verify market tickers are valid:
```bash
# Test if ticker exists on Kalshi
curl "https://api.elections.kalshi.com/trade-api/v2/markets/FED-25MAR-T4.75"
```

3. Check if markets are currently active (not expired)

4. Restart ingestor:
```bash
docker-compose restart ingestor
```

### Issue: Consumer Not Processing Messages

**Symptoms:**
```
# Empty or low consumer stats
curl http://localhost:8000/api/v1/stats/consumer
```

**Solutions:**

1. Check analysis service logs:
```bash
docker-compose logs analysis | grep -i consumer
```

2. Verify Redis stream has data:
```bash
docker-compose exec redis redis-cli XLEN market_ticks
```

3. Check for consumer group issues:
```bash
docker-compose exec redis redis-cli XINFO GROUPS market_ticks
```

4. Restart analysis service:
```bash
docker-compose restart analysis
```

### Issue: Frontend Not Showing Data

**Symptoms:**
- Dashboard loads but shows no markets
- "Connecting..." message persists
- No price updates

**Solutions:**

1. Check WebSocket connection in browser console (F12):
```javascript
// Should see WebSocket connection established
// Look for errors in console
```

2. Verify analysis service is running:
```bash
curl http://localhost:8000/health
```

3. Check if ticks are streaming:
```bash
curl http://localhost:8000/api/v1/ticks
# Should return array of tick objects
```

4. Verify market subscriptions configuration:
```bash
curl http://localhost:8000/api/v1/pairs
# Should return your configured subscriptions
```

### Issue: Invalid Market Ticker

**Symptoms:**
```
{"level":"warn","msg":"failed to subscribe to market"}
```

**Solutions:**

1. Verify ticker format is correct (use market ticker, not event or series ticker)

2. Check if market is active on Kalshi:
```bash
curl "https://api.elections.kalshi.com/trade-api/v2/markets/YOUR-TICKER"
```

3. Update `config/market_pairs.json` with valid tickers

4. Restart services:
```bash
docker-compose restart ingestor analysis
```

## Understanding the Data Pipeline

### Data Flow Architecture

1. **Exchange WebSockets**: Kalshi and Polymarket send real-time order book updates
2. **Go Ingestor**: Receives WebSocket messages, normalizes to common Tick format
3. **Redis Streams**: Ingestor publishes to `market_ticks` stream (msgpack encoding)
4. **Python Consumer**: Reads from stream, processes in batches
5. **Transform Layer**: Configured transforms are currently not applied (raw tick mode)
6. **Redis Pub/Sub**: Publishes tick updates to `tick:*` channels
7. **WebSocket API**: Broadcasts raw ticks to connected frontend clients
8. **Frontend**: Updates tick list and latency metrics in real-time

### Latency Tracking

Each tick contains three timestamps for latency analysis:

- `ts_source`: When the exchange generated the price update
- `ts_ingest`: When the ingestor received the WebSocket message
- `ts_emit`: When the analysis service published to frontend

The dashboard displays:
- Ingestion latency: `ts_ingest - ts_source`
- Processing latency: `ts_emit - ts_ingest`
- End-to-end latency: `ts_emit - ts_source`

### Metrics and Monitoring

Prometheus metrics are available at:
- Ingestor: http://localhost:9090/metrics
- Analysis: http://localhost:9091/metrics

Key metrics:
- `echoarb_messages_received_total{source="KALSHI"}`: Kalshi message count
- `echoarb_messages_received_total{source="POLYMARKET"}`: Polymarket message count
- `echoarb_ingest_latency_seconds`: Data freshness histogram
- `echoarb_connection_status{source="KALSHI"}`: Connection health (1 = connected, 0 = disconnected)
- `echoarb_errors_total`: Error counts by source and type

## Security Best Practices

1. **Never commit sensitive files to version control:**
   - `.env` file
   - `keys/` directory
   - Any files containing API credentials

2. **File permissions:**
   - Private key: `chmod 600 keys/kalshi_private_key.pem`
   - Config files: `chmod 644 config/market_pairs.json`

3. **API key rotation:**
   - Periodically generate new API keys on Kalshi
   - Update `.env` file with new Key ID
   - Replace private key file
   - Restart services

4. **Environment separation:**
   - Use different API keys for development, staging, and production
   - Never use production keys in development environments

5. **Monitoring:**
   - Set up alerts for authentication failures
   - Monitor error metrics for unusual patterns
   - Track connection uptime

## Next Steps

Once real data is flowing successfully:

1. **Add additional market subscriptions:**
   - Edit `config/market_pairs.json`
   - Restart services: `docker-compose restart`

2. **Alert thresholds (legacy):**
   - `alert_threshold` is ignored in raw tick mode

3. **Monitor performance:**
   - Track latency metrics in Prometheus
   - Optimize Redis consumer batch size if needed
   - Scale horizontally by adding consumer instances

4. **Set up Grafana dashboards:**
   - Start Grafana: `docker-compose --profile monitoring up -d`
   - Access: http://localhost:3001
   - Import pre-configured dashboards from `grafana/dashboards/`

5. **Enable historical analysis:**
   - Data is automatically stored in TimescaleDB
   - Query using SQL for backtesting and analysis
   - Create custom aggregations using continuous aggregates

## API Reference

### Kalshi API Documentation
https://trading-api.readme.io/reference/getting-started

Key concepts:
- Authentication uses RSA-PSS signing with SHA256
- Kalshi generates API keys and private keys for users
- WebSocket endpoint: `wss://api.elections.kalshi.com/trade-api/ws/v2`
- Market data includes bid/ask prices (in cents)
- Orderbook delta updates are pushed in real-time

### Polymarket API Documentation
https://docs.polymarket.com/

Key concepts:
- WebSocket endpoint: `wss://ws-subscriptions-clob.polymarket.com/ws`
- Token IDs identify specific markets
- Prices are normalized to 0-1 probability scale
- No authentication required for market data

## Support and Resources

- Kalshi Support: https://kalshi.com/support
- Polymarket Discord: https://discord.gg/polymarket
- API Documentation: http://localhost:8000/docs (when services running)

Your EchoArb system is now configured for real-time prediction market tick streaming.
