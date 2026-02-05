# EchoArb - Real-Time Prediction Market Arbitrage Scanner

A high-performance, real-time arbitrage detection system for prediction markets. Built with Go for low-latency data ingestion, Python/FastAPI for business logic, and Next.js for real-time visualization.

## Supported Platforms

- **Kalshi**: CFTC-regulated prediction market
- **Polymarket**: Decentralized prediction market

## Architecture

The system follows a three-tier architecture optimized for low latency and high throughput:

```
┌─────────────────────────────────────────────────┐
│           Go Ingestor (Port 9090)               │
│  ┌──────────────┐  ┌──────────────┐             │
│  │   Kalshi WS  │  │ Polymarket WS│             │
│  └──────┬───────┘  └──────┬───────┘             │
│         │                 │                     │
│         └────────┬────────┘                     │
│                  ▼                              │
│         ┌────────────────┐                      │
│         │ Redis Streams  │                      │
│         └────────┬───────┘                      |
└──────────────────┼──────────────────────────────┘
                   │
┌──────────────────┼──────────────────────────────┐
│         Python Analysis (Port 8000)             │
│         ┌────────▼───────┐                      │
│         │  Transform     │                      │
│         │  Layer         │                      │
│         └────────┬───────┘                      │
│                  ▼                              │
│         ┌────────────────┐                      │
│         │  Spread Calc   │                      │
│         └────────┬───────┘                      │
│                  │                              │
│         ┌────────▼───────┐                      │
│         │ TimescaleDB    │                      │
│         └────────────────┘                      │
└──────────────────┼──────────────────────────────┘
                   │ WebSocket
┌──────────────────▼──────────────────────────────┐
│         Next.js Frontend (Port 3000)            │
│         Real-time charts and alerts             │
└─────────────────────────────────────────────────┘
```

### Data Flow

1. **Ingestion Layer (Go)**: WebSocket connectors receive real-time order book updates from exchanges
2. **Message Broker (Redis Streams)**: Decouples ingestion from processing, provides durability
3. **Transform Layer (Python)**: Normalizes different market structures (binary, ranged) into comparable formats
4. **Spread Calculator (Python)**: Computes arbitrage opportunities between platform pairs
5. **Storage (TimescaleDB)**: Time-series database for historical analysis and backtesting
6. **WebSocket API (FastAPI)**: Pushes real-time updates to connected clients
7. **Frontend (Next.js)**: Visualizes live spreads, alerts, and latency metrics

## Technology Stack

| Layer       | Technology                    | Purpose                          |
|-------------|-------------------------------|----------------------------------|
| Ingestion   | Go 1.21, gorilla/websocket    | Low-latency data collection      |
| Analysis    | Python 3.11, FastAPI, Pandas  | Business logic and transforms    |
| Frontend    | Next.js 14, TypeScript        | Real-time visualization          |
| Storage     | Redis Streams, TimescaleDB    | Message queue and time-series    |
| Metrics     | Prometheus, Grafana           | Observability and monitoring     |
| Infra       | Docker, GitHub Actions        | Containerization and CI/CD       |

## Prerequisites

- Docker 20.10+ and Docker Compose 1.29+
- For local development (optional):
  - Go 1.21+
  - Python 3.11+
  - Node.js 20+

## Quick Start

### 1. Clone Repository

```bash
git clone https://github.com/dragonuber/echoarb.git
cd echoarb
```

### 2. Configure Environment

Copy the example environment file:

```bash
cp .env.example .env
```

Edit `.env` and configure the following variables:

```bash
# Kalshi API Credentials
KALSHI_API_KEY=         # API Key ID from Kalshi
KALSHI_PRIVATE_KEY_PATH=./keys/kalshi_private_key.pem

# Redis Configuration
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=

# Database Configuration
DB_HOST=timescaledb
DB_PORT=5432
DB_DATABASE=echoarb
DB_USER=postgres
DB_PASSWORD=changeme

# Service Ports
API_PORT=8000
METRICS_PORT=9090
```

### 3. Obtain Kalshi API Credentials

See [REAL_DATA_SETUP.md](REAL_DATA_SETUP.md) for detailed instructions on:
- Generating API keys from Kalshi
- Configuring market pairs
- Finding market tickers and token IDs

### 4. Configure Market Pairs

Edit `config/market_pairs.json` to specify which markets to track:

```json
{
  "pairs": [
    {
      "id": "fed-rate-march-2025",
      "description": "Federal Reserve interest rate decision March 2025",
      "kalshi_tickers": ["FED-25MAR-T4.75", "FED-25MAR-T5.00"],
      "kalshi_transform": "sum",
      "poly_token_id": "0x1234567890abcdef1234567890abcdef12345678",
      "poly_transform": "identity",
      "alert_threshold": 0.05
    }
  ]
}
```

### 5. Start Services

```bash
docker-compose up -d
```

### 6. Verify System Health

```bash
# Check service status
docker-compose ps

# View logs
docker-compose logs -f

# Test API endpoints
curl http://localhost:8000/health
curl http://localhost:8000/api/v1/spreads

# Access frontend
open http://localhost:3000
```

## Service Endpoints

| Service    | Endpoint                          | Description                  |
|------------|-----------------------------------|------------------------------|
| Frontend   | http://localhost:3000             | Real-time dashboard          |
| API        | http://localhost:8000             | FastAPI application          |
| API Docs   | http://localhost:8000/docs        | OpenAPI documentation        |
| Metrics    | http://localhost:9090/metrics     | Prometheus metrics (ingestor)|
| Metrics    | http://localhost:9091/metrics     | Prometheus metrics (analysis)|
| Grafana    | http://localhost:3001             | Metrics visualization        |

## Project Structure

```
echoarb/
├── ingestor/              # Go data ingestion service
│   ├── cmd/
│   │   └── ingestor/
│   │       └── main.go
│   └── internal/
│       ├── auth/          # Kalshi RSA-PSS authentication
│       ├── connectors/    # WebSocket connectors (Kalshi, Polymarket)
│       ├── metrics/       # Prometheus metrics
│       ├── models/        # Data models
│       ├── redis/         # Redis Stream publisher
│       └── retry/         # Retry logic with exponential backoff
├── analysis/              # Python analysis service
│   └── app/
│       ├── api/           # FastAPI routes and WebSocket handlers
│       ├── database/      # SQLAlchemy models
│       ├── models/        # Pydantic models
│       └── services/      # Business logic (transform, spread calc)
├── frontend/              # Next.js frontend
│   └── src/
│       ├── app/           # Next.js 14 app directory
│       ├── components/    # React components
│       ├── hooks/         # Custom hooks (WebSocket, API)
│       └── lib/           # Utilities and API client
└── config/                # Configuration files
    ├── market_pairs.json  # Market configuration
    └── prometheus.yml     # Prometheus scrape config
```

## Configuration

### Environment Variables

All services are configured via environment variables in `.env`. See `.env.example` for complete reference.

### Market Pairs Configuration

The `config/market_pairs.json` file defines which markets to track and how to transform prices:

- **kalshi_tickers**: Array of Kalshi market tickers (use array with "sum" transform for ranged contracts)
- **kalshi_transform**: How to combine multiple tickers ("sum", "identity", "inverse")
- **poly_token_id**: Polymarket token ID (found in browser DevTools Network tab)
- **poly_transform**: Usually "identity" for direct probability mapping
- **alert_threshold**: Minimum spread percentage to trigger alerts (e.g., 0.05 = 5%)

### Transform Strategies

The transform layer normalizes different market structures:

- **identity**: Use price as-is (1:1 mapping)
- **sum**: Add multiple Kalshi contracts (e.g., "4.75-5.00%" + ">5.00%" = ">4.75%")
- **inverse**: Flip probability (1 - price) for opposite outcomes

## Performance Characteristics

| Metric                 | Target   | Typical   |
|------------------------|----------|-----------|
| Ingestion Latency      | <20ms    | 12-18ms   |
| End-to-End Latency     | <100ms   | 50-90ms   |
| Messages/Second        | 1,000+   | 2,500+    |
| WebSocket Reconnection | <5s      | 2-3s      |

Latency breakdown:
- Network (exchange to server): 20-100ms
- Parsing and validation: 2-5ms
- Redis Stream publish: 0.5-1ms
- Transform and calculation: 5-10ms
- WebSocket push to frontend: 10-50ms

## Monitoring

### Prometheus Metrics

Access metrics at:
- Ingestor: http://localhost:9090/metrics
- Analysis: http://localhost:9091/metrics

Key metrics:
- `echoarb_messages_received_total`: Messages received by source
- `echoarb_ingest_latency_seconds`: Data freshness
- `echoarb_connection_status`: Connection health (1 = connected)
- `echoarb_errors_total`: Error counts by source and type
- `echoarb_processing_time_seconds`: Processing duration histogram

### Grafana Dashboards

Start Grafana with monitoring profile:

```bash
docker-compose --profile monitoring up -d
```

Access at http://localhost:3001 (default credentials: admin/admin)

Pre-configured dashboards available in `grafana/dashboards/`

## Development

### Running Individual Services

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

### Testing

```bash
# Go tests
cd ingestor
go test ./...

# Python tests
cd analysis
pytest

# Frontend tests
cd frontend
npm test
```

### Code Quality

```bash
# Go
cd ingestor
go fmt ./...
go vet ./...
golangci-lint run

# Python
cd analysis
black .
ruff check .
mypy .

# TypeScript
cd frontend
npm run lint
npm run type-check
```

## Deployment

### Production Build

```bash
docker-compose -f docker-compose.yml -f docker-compose.prod.yml build
```

### Environment-Specific Configuration

Create environment-specific compose files:
- `docker-compose.dev.yml`: Development overrides
- `docker-compose.staging.yml`: Staging configuration
- `docker-compose.prod.yml`: Production configuration

### Security Considerations

- Never commit `.env` or `keys/` directory to version control
- Use different Kalshi API keys for dev/staging/production
- Set private key file permissions to 600 (`chmod 600 keys/kalshi_private_key.pem`)
- Rotate API keys periodically
- Monitor error logs for authentication failures
- Use read-only volume mounts for keys in production

## Troubleshooting

See [REAL_DATA_SETUP.md](REAL_DATA_SETUP.md) for detailed troubleshooting steps.

Common issues:

**Ingestor fails to authenticate with Kalshi**
- Verify `KALSHI_API_KEY` matches the Key ID from Kalshi
- Verify `KALSHI_PRIVATE_KEY_PATH` points to the correct PEM file
- Check private key file permissions (should be 600)
- Ensure private key is in valid PEM format

**No data flowing through system**
- Check ingestor logs: `docker-compose logs -f ingestor`
- Verify market tickers in `config/market_pairs.json` are active
- Check Redis stream length: `docker-compose exec redis redis-cli XLEN market_ticks`
- Verify consumer is running: `docker-compose logs -f analysis | grep consumer`

**Frontend not showing data**
- Check WebSocket connection in browser console
- Verify analysis service is running: `curl http://localhost:8000/health`
- Check for spreads: `curl http://localhost:8000/api/v1/spreads`
- Verify market pairs are configured correctly

## Documentation

- [REAL_DATA_SETUP.md](REAL_DATA_SETUP.md): Complete setup guide for real market data
- [MANIFOLD_REMOVED.md](MANIFOLD_REMOVED.md): Documentation of Manifold Markets removal
- [API Documentation](http://localhost:8000/docs): OpenAPI specification (when services running)

## License

[Your License Here]

## Acknowledgments

- Kalshi API: https://trading-api.readme.io/reference/getting-started
- Polymarket API: https://docs.polymarket.com/
