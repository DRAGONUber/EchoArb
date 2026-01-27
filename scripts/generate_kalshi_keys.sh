#!/bin/bash
# scripts/generate_kalshi_keys.sh
# Generate RSA keypair for Kalshi API authentication

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Kalshi RSA Keypair Generator${NC}"
echo "=========================================="
echo ""

# Check if openssl is installed
if ! command -v openssl &> /dev/null; then
    echo -e "${RED}Error: openssl is not installed${NC}"
    echo "Please install openssl first:"
    echo "  macOS: brew install openssl"
    echo "  Ubuntu: sudo apt-get install openssl"
    exit 1
fi

# Create keys directory
KEYS_DIR="./keys"
mkdir -p "$KEYS_DIR"

echo -e "${YELLOW}Step 1: Generating RSA private key (2048 bits)...${NC}"
openssl genrsa -out "$KEYS_DIR/kalshi_private_key.pem" 2048

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Private key generated: $KEYS_DIR/kalshi_private_key.pem${NC}"
else
    echo -e "${RED}✗ Failed to generate private key${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}Step 2: Extracting public key...${NC}"
openssl rsa -in "$KEYS_DIR/kalshi_private_key.pem" -pubout -out "$KEYS_DIR/kalshi_public_key.pem"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Public key extracted: $KEYS_DIR/kalshi_public_key.pem${NC}"
else
    echo -e "${RED}✗ Failed to extract public key${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}Step 3: Setting file permissions...${NC}"
chmod 600 "$KEYS_DIR/kalshi_private_key.pem"
chmod 644 "$KEYS_DIR/kalshi_public_key.pem"
echo -e "${GREEN}✓ Permissions set (private: 600, public: 644)${NC}"

echo ""
echo "=========================================="
echo -e "${GREEN}Keypair generated successfully!${NC}"
echo ""
echo "Next steps:"
echo "1. Copy the public key to your clipboard:"
echo -e "   ${YELLOW}cat $KEYS_DIR/kalshi_public_key.pem | pbcopy${NC} (macOS)"
echo -e "   ${YELLOW}cat $KEYS_DIR/kalshi_public_key.pem | xclip -selection clipboard${NC} (Linux)"
echo ""
echo "2. Upload the public key to your Kalshi account:"
echo "   - Go to https://kalshi.com/account/api"
echo "   - Create a new API key"
echo "   - Paste the public key"
echo "   - Save the API Key ID"
echo ""
echo "3. Update your .env file:"
echo -e "   ${YELLOW}KALSHI_API_KEY=<your-api-key-id>${NC}"
echo -e "   ${YELLOW}KALSHI_PRIVATE_KEY_PATH=./keys/kalshi_private_key.pem${NC}"
echo ""
echo -e "${RED}IMPORTANT: Keep your private key secure and NEVER commit it to version control!${NC}"
echo ""
