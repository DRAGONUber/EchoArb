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
mkdir -p data/redis
mkdir -p data/postgres
mkdir -p data/grafana

echo -e "${GREEN}✓ Directories created${NC}"

# Create .env file if it doesn't exist
if [ ! -f .env ]; then
    echo -e "\n${YELLOW}Creating .env file from template...${NC}"
    
    cat > .env << 'EOF'
# Environment
ENVIRONMENT=development
LOG_LEVEL=info

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# Database
DATABASE_URL=postgresql+asyncpg://echoarb:echoarb_pass@localhost:5432/echoarb

# Kalshi (YOU MUST FILL THESE IN)
KALSHI_API_KEY=your_kalshi_api_key_here
KALSHI_PRIVATE_KEY_PATH=./keys/kalshi_private_key.pem
KALSHI_WS_URL=wss://api.elections.kalshi.com/trade-api/ws/v2

# Polymarket
POLY_WS_URL=wss://ws-subscriptions-clob.polymarket.com/ws

# Metrics
METRICS_PORT=9090

# Config
CONFIG_PATH=./config/market_pairs.json
EOF

    echo -e "${GREEN}✓ .env file created${NC}"
    echo -e "${YELLOW}⚠  Please edit .env and add your Kalshi API credentials${NC}"
else
    echo -e "${GREEN}✓ .env file already exists${NC}"
fi

# Create sample market pairs config
if [ ! -f config/market_pairs.json ]; then
    echo -e "\n${YELLOW}Creating sample market_pairs.json...${NC}"
    
    cat > config/market_pairs.json << 'EOF'
{
  "pairs": [
    {
      "id": "fed-rate-example",
      "description": "Example Fed rate market pair",
      "kalshi": {
        "ticker": "FED-25MAR-T4.75"
      },
      "polymarket": {
        "token_id": "0x0000000000000000000000000000000000000000"
      },
      "manifold": {
        "slug": "will-fed-cut-rates-march-2025"
      }
    }
  ]
}
EOF

    echo -e "${GREEN}✓ Sample market_pairs.json created${NC}"
    echo -e "${YELLOW}⚠  Please edit config/market_pairs.json with real market IDs${NC}"
else
    echo -e "${GREEN}✓ market_pairs.json already exists${NC}"
fi

# Generate Kalshi keys if they don't exist
if [ ! -f keys/kalshi_private_key.pem ]; then
    echo -e "\n${YELLOW}Generating Kalshi RSA keypair...${NC}"
    
    # Generate private key
    openssl genrsa -out keys/kalshi_private_key.pem 4096
    
    # Generate public key
    openssl rsa -in keys/kalshi_private_key.pem -pubout -out keys/kalshi_public_key.pem
    
    # Set proper permissions
    chmod 600 keys/kalshi_private_key.pem
    chmod 644 keys/kalshi_public_key.pem
    
    echo -e "${GREEN}✓ RSA keypair generated${NC}"
    echo -e "${YELLOW}⚠  Upload keys/kalshi_public_key.pem to Kalshi dashboard${NC}"
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
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Edit .env and add your Kalshi API credentials"
echo "2. Upload keys/kalshi_public_key.pem to Kalshi dashboard"
echo "3. Edit config/market_pairs.json with real market IDs"
echo "4. Run 'make dev' to start development environment"
echo ""
echo -e "${GREEN}Quick start:${NC}"
echo "  make dev              # Start all services"
echo "  make logs             # View logs"
echo "  make test             # Run tests"
echo "  make help             # Show all commands"
echo ""
echo -e "${GREEN}Documentation:${NC}"
echo "  Ingestor metrics: http://localhost:9090/metrics"
echo "  Analysis API:     http://localhost:8000/docs"
echo "  Frontend:         http://localhost:3000"
echo "  Grafana:          http://localhost:3001"
echo ""
