#!/bin/bash
# scripts/setup.sh - Initial project setup

set -e

GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}======================================"
echo "EchoArb - Initial Setup"
echo -e "======================================${NC}"

# Check required tools
echo -e "\n${YELLOW}Checking required tools...${NC}"

command -v docker >/dev/null 2>&1 || {
    echo -e "${RED}Error: docker is not installed${NC}" >&2
    exit 1
}

command -v docker-compose >/dev/null 2>&1 || {
    echo -e "${RED}Error: docker-compose is not installed${NC}" >&2
    exit 1
}

command -v go >/dev/null 2>&1 || {
    echo -e "${YELLOW}Warning: Go is not installed (needed for local development)${NC}"
}

command -v python3 >/dev/null 2>&1 || {
    echo -e "${YELLOW}Warning: Python 3 is not installed (needed for local development)${NC}"
}

command -v node >/dev/null 2>&1 || {
    echo -e "${YELLOW}Warning: Node.js is not installed (needed for local development)${NC}"
}

echo -e "${GREEN}✓ Required tools check complete${NC}"

# Create necessary directories
echo -e "\n${YELLOW}Creating directory structure...${NC}"

mkdir -p keys
mkdir -p config
mkdir -p logs

echo -e "${GREEN}✓ Directories created${NC}"

# Create .env file if it doesn't exist
if [ ! -f .env ]; then
    echo -e "\n${YELLOW}Creating .env file from template...${NC}"

    cat > .env << 'EOF'
# Environment
ENVIRONMENT=development
LOG_LEVEL=INFO

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_STREAM_NAME=market_ticks
REDIS_CONSUMER_GROUP=tick_consumers
REDIS_CONSUMER_NAME=worker_1

# Kalshi (Optional - leave empty to run Polymarket only)
KALSHI_API_KEY=
KALSHI_PRIVATE_KEY_PATH=
KALSHI_WS_URL=wss://api.elections.kalshi.com/trade-api/ws/v2

# Polymarket
POLY_WS_URL=wss://ws-subscriptions-clob.polymarket.com/ws

# API
API_PORT=8000

# Metrics
METRICS_ENABLED=true
METRICS_PORT=9091

# Frontend
NEXT_PUBLIC_API_URL=http://localhost:8000
NEXT_PUBLIC_WS_URL=ws://localhost:8000
EOF

    echo -e "${GREEN}✓ .env file created${NC}"
    echo -e "${YELLOW}⚠  To enable Kalshi, add your API credentials to .env${NC}"
else
    echo -e "${GREEN}✓ .env file already exists${NC}"
fi

# Generate Kalshi keys if they don't exist (optional)
if [ ! -f keys/kalshi_private_key.pem ]; then
    echo -e "\n${YELLOW}Generating Kalshi RSA keypair (optional for Kalshi integration)...${NC}"

    # Generate private key
    openssl genrsa -out keys/kalshi_private_key.pem 4096

    # Generate public key
    openssl rsa -in keys/kalshi_private_key.pem -pubout -out keys/kalshi_public_key.pem

    # Set proper permissions
    chmod 600 keys/kalshi_private_key.pem
    chmod 644 keys/kalshi_public_key.pem

    echo -e "${GREEN}✓ RSA keypair generated${NC}"
    echo -e "${YELLOW}⚠  To use Kalshi: Upload keys/kalshi_public_key.pem to Kalshi dashboard${NC}"
else
    echo -e "${GREEN}✓ Kalshi keys already exist${NC}"
fi

# Pull Docker images
echo -e "\n${YELLOW}Pulling Docker images (this may take a few minutes)...${NC}"
docker-compose pull

echo -e "${GREEN}✓ Docker images pulled${NC}"

# Install Go dependencies (if Go is installed)
if command -v go >/dev/null 2>&1; then
    echo -e "\n${YELLOW}Installing Go dependencies...${NC}"
    cd ingestor && go mod download && cd ..
    echo -e "${GREEN}✓ Go dependencies installed${NC}"
fi

# Create gitignore if it doesn't exist
if [ ! -f .gitignore ]; then
    echo -e "\n${YELLOW}Creating .gitignore...${NC}"

    cat > .gitignore << 'EOF'
# Environment
.env
.env.local
.env.production

# Keys (NEVER commit these!)
keys/*.pem
*.key
*.crt

# Logs
logs/
*.log

# Data directories
data/

# Go
ingestor/bin/
ingestor/vendor/
*.exe
*.test
coverage.out

# Python
analysis/__pycache__/
analysis/.pytest_cache/
analysis/venv/
analysis/htmlcov/
*.pyc
*.pyo
*.pyd
.Python
.coverage

# Node.js
frontend/node_modules/
frontend/.next/
frontend/out/
frontend/build/

# IDE
.vscode/
.idea/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db
EOF

    echo -e "${GREEN}✓ .gitignore created${NC}"
fi

# Final instructions
echo -e "\n${GREEN}======================================"
echo "Setup Complete!"
echo -e "======================================${NC}"
echo ""
echo -e "${YELLOW}Quick start (Polymarket only):${NC}"
echo "  docker-compose up -d    # Start services"
echo ""
echo -e "${YELLOW}To enable Kalshi:${NC}"
echo "  1. Upload keys/kalshi_public_key.pem to Kalshi dashboard"
echo "  2. Edit .env and add KALSHI_API_KEY and KALSHI_PRIVATE_KEY_PATH"
echo "  3. Restart: docker-compose up -d --build ingestor"
echo ""
echo -e "${GREEN}Endpoints:${NC}"
echo "  Analysis API:     http://localhost:8000/docs"
echo "  Frontend:         http://localhost:3000"
echo "  Ingestor metrics: http://localhost:9090/metrics"
echo ""
