#!/bin/bash
# scripts/test_data.sh
# Generate test data for EchoArb dashboard

API_URL="http://localhost:8000"

echo "🧪 Injecting test data into EchoArb..."
echo ""

# Fed Rate March 2025 - Kalshi contracts
echo "📊 Adding Kalshi data..."
curl -s -X POST "$API_URL/api/v1/debug/update_price" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "KALSHI",
    "contract_id": "FED-25MAR-T4.75",
    "price": 0.35
  }' | jq .

sleep 0.5

curl -s -X POST "$API_URL/api/v1/debug/update_price" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "KALSHI",
    "contract_id": "FED-25MAR-T5.00",
    "price": 0.20
  }' | jq .

sleep 0.5

# Polymarket data
echo ""
echo "📊 Adding Polymarket data..."
curl -s -X POST "$API_URL/api/v1/debug/update_price" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "POLYMARKET",
    "contract_id": "0x1234567890abcdef1234567890abcdef12345678",
    "price": 0.58
  }' | jq .

sleep 0.5

# Manifold data
echo ""
echo "📊 Adding Manifold data..."
curl -s -X POST "$API_URL/api/v1/debug/update_price" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "MANIFOLD",
    "contract_id": "will-the-fed-cut-rates-in-march-2025",
    "price": 0.52
  }' | jq .

sleep 1

# Check spreads
echo ""
echo "📈 Current spreads:"
curl -s "$API_URL/api/v1/spreads" | jq .

echo ""
echo "✅ Test data injected! Check the dashboard at http://localhost:3000"
