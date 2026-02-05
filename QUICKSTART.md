# EchoArb - Quick Start Guide

## Get Started in 5 Minutes

### Prerequisites
- Docker and Docker Compose installed
- Kalshi account (for real market data)

### Step 1: Clone and Configure

```bash
# Clone repository
git clone https://github.com/dragonuber/echoarb.git
cd echoarb

# Copy environment template
cp .env.example .env
```

### Step 2: Obtain Kalshi API Credentials

1. Visit https://kalshi.com/account/profile
2. Click "Create New API Key"
3. Kalshi will generate an RSA keypair for you
4. Copy the **Private Key** (shown only once)
5. Save it to `keys/kalshi_private_key.pem`
6. Copy the **Key ID** (20-character identifier)
7. Add the Key ID to your `.env` file:

```bash
mkdir -p keys
# Paste the private key into this file
nano keys/kalshi_private_key.pem

# Set correct permissions
chmod 600 keys/kalshi_private_key.pem

# Edit .env and add your Key ID
nano .env
# KALSHI_API_KEY=your-key-id-here
```

See [REAL_DATA_SETUP.md](REAL_DATA_SETUP.md) for detailed instructions.

### Step 3: Configure Market Subscriptions

Edit `config/market_pairs.json` to specify which markets to subscribe to:

```bash
nano config/market_pairs.json
```

Example configuration:

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
    }
  ]
}
```

Note: `kalshi_transform`, `poly_transform`, and `alert_threshold` are ignored in raw tick mode.

### Step 4: Start Services

```bash
# Build and start all services
docker-compose up -d

# View logs
docker-compose logs -f
```

Services start in this order:
1. Redis (message broker)
2. TimescaleDB (time-series database)
3. Go Ingestor (WebSocket data collection)
4. Python Analysis (REST API and business logic)
5. Next.js Frontend (dashboard)

### Step 5: Access the System

Open your browser to:

- **Dashboard**: http://localhost:3000
- **API Documentation**: http://localhost:8000/docs
- **API Health**: http://localhost:8000/health
- **Ingestor Metrics**: http://localhost:9090/metrics
- **Analysis Metrics**: http://localhost:9091/metrics

## Verify System Operation

### Check Service Health

```bash
# Check all services are running
docker-compose ps

# View ingestor logs
docker-compose logs -f ingestor
# Should see: "Connected to Kalshi" and "Connected to Polymarket"

# Test API
curl http://localhost:8000/health
# Should return: {"status":"healthy"}

# View current ticks
curl http://localhost:8000/api/v1/ticks | jq .
```

### Check Data Flow

```bash
# Check Redis stream
docker-compose exec redis redis-cli XLEN market_ticks
# Should return number > 0 if data is flowing

# Check consumer stats
curl http://localhost:8000/api/v1/stats/consumer | jq .
```

## Development Mode

For development with hot reload:

```bash
# Start infrastructure only
docker-compose up redis timescaledb

# Run ingestor locally
cd ingestor
go run cmd/ingestor/main.go

# Run analysis locally
cd analysis
pip install -r requirements.txt
uvicorn app.main:app --reload

# Run frontend locally
cd frontend
npm install
npm run dev
```

## Test Without Real Data

For testing the system without Kalshi credentials, use the debug endpoint to inject test data:

```bash
curl -X POST "http://localhost:8000/api/v1/debug/update_price?source=KALSHI&contract_id=FED-25MAR-T4.75&price=0.35"

curl -X POST "http://localhost:8000/api/v1/debug/update_price?source=POLYMARKET&contract_id=0x1234567890abcdef1234567890abcdef12345678&price=0.40"
```

The dashboard should now show raw ticks for the injected contracts.

## Troubleshooting

### Authentication Failure

**Symptoms:**
```
{"level":"error","msg":"failed to get auth headers"}
{"level":"fatal","msg":"failed to decode PEM block"}
```

**Solutions:**
1. Verify private key is in valid PEM format:
```bash
head -1 keys/kalshi_private_key.pem
# Should show: -----BEGIN PRIVATE KEY----- or -----BEGIN RSA PRIVATE KEY-----
```

2. Check file permissions:
```bash
ls -la keys/kalshi_private_key.pem
# Should be: -rw------- (600)
chmod 600 keys/kalshi_private_key.pem
```

3. Verify API Key ID in .env matches Kalshi

### Port Already in Use

Change ports in `docker-compose.yml` or stop conflicting services:

```bash
# Check what's using port 8000
lsof -i :8000

# Kill process
kill -9 <PID>
```

### Database Connection Issues

```bash
# Check TimescaleDB is running
docker-compose ps timescaledb

# View logs
docker-compose logs timescaledb

# Restart service
docker-compose restart timescaledb
```

### Redis Connection Issues

```bash
# Test Redis connectivity
docker-compose exec redis redis-cli ping
# Should return: PONG

# Check stream exists
docker-compose exec redis redis-cli XINFO STREAM market_ticks
```

### No Data in Dashboard

1. Check ingestor is connected:
```bash
docker-compose logs ingestor | grep -i "connected"
```

2. Verify market tickers are valid:
```bash
curl "https://api.elections.kalshi.com/trade-api/v2/markets/FED-25MAR-T4.75"
```

3. Check Redis stream has data:
```bash
docker-compose exec redis redis-cli XLEN market_ticks
```

4. Verify analysis service is processing:
```bash
docker-compose logs analysis | grep -i "consumer"
```

## Useful Commands

```bash
# Stop all services
docker-compose down

# Stop and remove volumes (fresh start)
docker-compose down -v

# Rebuild all services
docker-compose build --no-cache

# Rebuild specific service
docker-compose build analysis

# View logs for specific service
docker-compose logs -f analysis

# Access container shell
docker-compose exec analysis /bin/bash

# Run with monitoring (Prometheus + Grafana)
docker-compose --profile monitoring up -d
# Access Grafana at http://localhost:3001 (admin/admin)

# Restart specific service
docker-compose restart ingestor
```

## Architecture Overview

```
┌─────────────────┐
│  Kalshi API     │
│  Polymarket API │──┐
└─────────────────┘  │
                     │ WebSocket
                     ▼
              ┌─────────────┐
              │ Go Ingestor │ (High-performance data collection)
              └─────────────┘
                     │
                     │ Redis Streams (msgpack encoded)
                     ▼
              ┌─────────────┐
              │    Redis    │ (Message broker)
              └─────────────┘
                     │
                     ▼
         ┌──────────────────────┐
         │  Python Analysis     │ (FastAPI backend)
         │  - Tick Streaming    │
         │  - REST API          │
         │  - WebSocket Server  │
         └──────────────────────┘
                     │
        ┌────────────┴────────────┐
        │                         │
        ▼                         ▼
┌──────────────┐         ┌──────────────┐
│ TimescaleDB  │         │  Next.js UI  │
│ (Historical) │         │  (Dashboard) │
└──────────────┘         └──────────────┘
```

## System Components

**Go Ingestor (Port 9090):**
- WebSocket clients for Kalshi and Polymarket
- Sub-20ms message processing
- Publishes to Redis Streams
- Prometheus metrics export

**Python Analysis (Port 8000):**
- Redis Stream consumer with consumer groups
- REST API with OpenAPI documentation
- WebSocket server for raw tick updates

**Next.js Frontend (Port 3000):**
- Real-time dashboard with WebSocket updates
- Tick list and latency monitoring

**Redis (Port 6379):**
- Streams for message queue
- Pub/Sub for real-time notifications
- Minimal latency overhead

**TimescaleDB (Port 5433):**
- Time-series optimized PostgreSQL
- Historical price data

## What's Working

- Docker containerization with multi-stage builds
- Redis Streams message broker
- Go WebSocket data ingestion (Kalshi, Polymarket)
- Python FastAPI backend with async processing
- Next.js real-time dashboard
- Prometheus metrics collection
- Health checks on all services
- Auto-reconnection with exponential backoff
- Comprehensive error handling
- Raw tick streaming mode enabled

## Next Steps

1. **Configure Real Markets**: Edit `config/market_pairs.json` with actual market tickers
2. **Monitor Performance**: Check Prometheus metrics at http://localhost:9090/metrics
3. **View Historical Data**: Query TimescaleDB for backtesting
4. **Enable Grafana**: Run with `--profile monitoring` for visualization

## Documentation

- [README.md](README.md): Complete project overview
- [REAL_DATA_SETUP.md](REAL_DATA_SETUP.md): Detailed setup instructions
- [API Docs](http://localhost:8000/docs): Interactive API documentation (when running)

Your EchoArb system is now running. Happy trading!
