.PHONY: setup dev test clean

setup:
	@echo "Creating keys directory..."
	@mkdir -p keys
	@echo "Generating Kalshi Keys..."
	@openssl genrsa -out keys/kalshi_private_key.pem 4096
	@openssl rsa -in keys/kalshi_private_key.pem -pubout -out keys/kalshi_public_key.pem
	@chmod 600 keys/kalshi_private_key.pem
	@echo "Done. Upload keys/kalshi_public_key.pem to Kalshi Dashboard."

dev:
	docker-compose up --build

test:
	cd ingestor && go test ./...
	cd analysis && pytest

clean:
	docker-compose down -v