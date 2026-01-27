# EchoArb - Quick Start Guide

## 🚀 Get Started in 5 Minutes

### Prerequisites
- Docker and Docker Compose installed
- Kalshi API account (for real data)

### Step 1: Setup Environment

```bash
# Copy environment template
cp .env.example .env

# Edit .env and add your Kalshi API key
nano .env
```

### Step 2: Generate Kalshi Keys (Optional - for real trading data)

```bash
# Generate RSA keypair
./scripts/generate_kalshi_keys.sh

# Upload the public key to Kalshi at https://kalshi.com/account/api
# Add the API Key ID to your .env file
```

### Step 3: Start the Stack

```bash
# Start all services
docker-compose up
```

That's it! The services will start in this order:
1. Redis (message broker)
2. TimescaleDB (database)
3. Go Ingestor (data collection)
4. Python Analysis (backend API)
5. Next.js Frontend (dashboard)

### Step 4: Access the Dashboard

Open your browser to:
- **Dashboard**: http://localhost:3000
- **API Docs**: http://localhost:8000/docs
- **API**: http://localhost:8000
- **Metrics**: http://localhost:9090/metrics (Ingestor)
- **Metrics**: http://localhost:9091/metrics (Analysis)

## 🛠️ Development Mode

For development with hot reload:

```bash
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up
```

## 📊 Test Without Real Data

You can test the system without Kalshi keys:

```bash
# The system will load example market pairs from config/market_pairs.json
# Use the debug endpoint to manually add test data:

curl -X POST http://localhost:8000/api/v1/debug/update_price \
  -H "Content-Type: application/json" \
  -d '{
    "source": "KALSHI",
    "contract_id": "FED-25MAR-T4.75",
    "price": 0.35
  }'
```

## 🐛 Troubleshooting

### "Empty Dockerfile" Error
All Dockerfiles are now populated. Try:
```bash
docker-compose down
docker-compose build --no-cache
docker-compose up
```

### Port Already in Use
Change ports in `docker-compose.yml` or stop conflicting services:
```bash
# Check what's using port 8000
lsof -i :8000
```

### Database Connection Issues
```bash
# Check TimescaleDB is running
docker-compose ps timescaledb

# View logs
docker-compose logs timescaledb
```

### Redis Connection Issues
```bash
# Test Redis connectivity
docker-compose exec redis redis-cli ping
# Should return: PONG
```

## 📝 Next Steps

1. **Add Real Market Pairs**: Edit `config/market_pairs.json`
2. **Setup Database**: Run `python scripts/seed_db.py`
3. **Monitor Metrics**: Access Prometheus at http://localhost:9092 (with profile)
4. **View Logs**: `docker-compose logs -f [service_name]`

## 🔄 Useful Commands

```bash
# Stop all services
docker-compose down

# Stop and remove volumes (fresh start)
docker-compose down -v

# Rebuild specific service
docker-compose build analysis

# View logs
docker-compose logs -f analysis

# Access container shell
docker-compose exec analysis /bin/bash

# Run with monitoring (Prometheus + Grafana)
docker-compose --profile monitoring up
```

## 📚 Architecture Overview

```
┌─────────────────┐
│  Kalshi API     │
│  Polymarket API │──┐
│  Manifold API   │  │
└─────────────────┘  │
                     ▼
              ┌─────────────┐
              │ Go Ingestor │ (WebSocket clients)
              └─────────────┘
                     │
                     ▼
              ┌─────────────┐
              │    Redis    │ (Streams + Pub/Sub)
              └─────────────┘
                     │
                     ▼
         ┌──────────────────────┐
         │  Python Analysis     │ (FastAPI)
         │  - REST API          │
         │  - WebSocket Server  │
         │  - Spread Calculator │
         └──────────────────────┘
                     │
        ┌────────────┴────────────┐
        ▼                         ▼
┌──────────────┐         ┌──────────────┐
│ TimescaleDB  │         │  Next.js UI  │
│ (Historical) │         │  (Dashboard) │
└──────────────┘         └──────────────┘
```

## 🎯 What's Working

✅ Docker containerization
✅ Redis message broker
✅ Go WebSocket data ingestion
✅ Python FastAPI backend
✅ Next.js real-time dashboard
✅ Prometheus metrics
✅ Health checks on all services
✅ Auto-reconnection logic
✅ Error handling throughout

Enjoy using EchoArb! 🎉
