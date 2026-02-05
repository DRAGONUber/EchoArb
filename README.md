# EchoArb

Real-time tick streaming from Kalshi and Polymarket prediction markets.

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Polymarket    │     │     Kalshi      │     │    Frontend     │
│   WebSocket     │     │   WebSocket     │     │   (Next.js)     │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         └───────────┬───────────┘                       │
                     │                                   │
              ┌──────▼──────┐                           │
              │   Ingestor  │                           │
              │    (Go)     │                           │
              └──────┬──────┘                           │
                     │                                  │
              ┌──────▼──────┐                          │
              │    Redis    │◄─────────────────────────┘
              │   Streams   │        WebSocket
              └──────┬──────┘
                     │
              ┌──────▼──────┐
              │  Analysis   │
              │  (FastAPI)  │
              └─────────────┘
```

## Quick Start

```bash
# Start all services (Polymarket by default, Kalshi optional)
docker-compose up -d

# View logs
docker-compose logs -f ingestor

# Access endpoints
# - Frontend: http://localhost:3000
# - API Docs: http://localhost:8000/docs
```

## Enable Kalshi (Optional)

1. Generate keys:
   ```bash
   mkdir -p keys
   openssl genrsa -out keys/kalshi_private_key.pem 4096
   openssl rsa -in keys/kalshi_private_key.pem -pubout -out keys/kalshi_public_key.pem
   ```

2. Upload `keys/kalshi_public_key.pem` to Kalshi dashboard

3. Add to `.env`:
   ```
   KALSHI_API_KEY=your_api_key
   KALSHI_PRIVATE_KEY_PATH=./keys/kalshi_private_key.pem
   ```

4. Rebuild: `docker-compose up -d --build ingestor`

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/ticks` | Recent ticks (query: `limit`, `source`) |
| `GET /api/v1/stats/consumer` | Consumer statistics |
| `GET /api/v1/stats/stream` | Redis stream statistics |
| `WS /ws/spreads` | Real-time tick stream |
| `WS /ws/ticks` | Real-time tick stream (pub/sub) |

## Project Structure

```
EchoArb/
├── ingestor/           # Go WebSocket ingestor
│   ├── cmd/ingestor/   # Main entry point
│   └── internal/       # Connectors, config, auth
├── analysis/           # Python FastAPI backend
│   └── app/            # API routes, WebSocket, consumer
├── frontend/           # Next.js dashboard
└── docker-compose.yml  # Service orchestration
```

## Technology Stack

| Layer | Technology | Purpose |
|-------|------------|---------|
| Ingestion | Go, gorilla/websocket | Low-latency data collection |
| Streaming | Python, FastAPI | WebSocket/REST API |
| Frontend | Next.js, TypeScript | Real-time visualization |
| Messaging | Redis Streams | Message queue |
| Metrics | Prometheus | Observability |

## Development

```bash
# Start infrastructure only
docker-compose up -d redis

# Run ingestor locally
cd ingestor && go run cmd/ingestor/main.go

# Run analysis locally
cd analysis && pip install -r requirements.txt && uvicorn app.main:app --reload

# Run frontend locally
cd frontend && npm install && npm run dev
```

## Monitoring

Start with monitoring profile:
```bash
docker-compose --profile monitoring up -d
```

- Grafana: http://localhost:3001 (admin/admin)
- Prometheus: http://localhost:9092
