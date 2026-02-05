// internal/connectors/polymarket.go
package connectors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/dragonuber/echoarb/ingestor/internal/config"
	"github.com/dragonuber/echoarb/ingestor/internal/models"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type PolymarketConnector struct {
	config     *config.Config
	logger     *zap.SugaredLogger
	conn       *websocket.Conn
	mu         sync.RWMutex
	lastPrices sync.Map
	msgChan    chan<- models.Tick
}

func NewPolymarketConnector(cfg *config.Config, logger *zap.SugaredLogger, msgChan chan<- models.Tick) *PolymarketConnector {
	return &PolymarketConnector{
		config:  cfg,
		logger:  logger,
		msgChan: msgChan,
	}
}

func (p *PolymarketConnector) Start() error {
	p.logger.Info("Starting Polymarket connector")
	return p.connect()
}

func (p *PolymarketConnector) connect() error {
	p.logger.Info("Connecting to Polymarket WebSocket...")

	// 1. Connect
	conn, _, err := websocket.DefaultDialer.Dial(p.config.PolyWSURL, nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	p.mu.Lock()
	p.conn = conn
	p.mu.Unlock()

	p.logger.Info("Connected to Polymarket")

	// 2. Subscribe (Firehose)
	if err := p.subscribe(conn); err != nil {
		return err
	}

	// 3. Read loop
	go p.readLoop()
	return nil
}

type PolymarketMarket struct {
	ClobTokenIDs string `json:"clobTokenIds"`
}

func (p *PolymarketConnector) fetchActiveMarkets() ([]string, error) {
	p.logger.Info("Fetching all active Polymarket tokens via Gamma API...")

	url := "https://gamma-api.polymarket.com/markets?closed=false&limit=500"
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("api returned status: %s", resp.Status)
	}

	var markets []PolymarketMarket
	if err := json.NewDecoder(resp.Body).Decode(&markets); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	var allTokenIDs []string
	for _, m := range markets {
		var ids []string
		if m.ClobTokenIDs != "" {
			if err := json.Unmarshal([]byte(m.ClobTokenIDs), &ids); err == nil {
				allTokenIDs = append(allTokenIDs, ids...)
			}
		}
	}

	if len(allTokenIDs) == 0 {
		return nil, fmt.Errorf("discovery returned no active tokens")
	}

	p.logger.Infof("Discovered %d active Polymarket tokens", len(allTokenIDs))
	return allTokenIDs, nil
}

func (p *PolymarketConnector) subscribe(conn *websocket.Conn) error {
	tokenIDs, err := p.fetchActiveMarkets()
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	p.logger.Infof("Subscribing to %d Polymarket tokens...", len(tokenIDs))

	// Subscribe in batches (Polymarket accepts arrays of assets_ids)
	batchSize := 100
	for i := 0; i < len(tokenIDs); i += batchSize {
		end := i + batchSize
		if end > len(tokenIDs) {
			end = len(tokenIDs)
		}
		batch := tokenIDs[i:end]

		subscribeMsg := map[string]interface{}{
			"assets_ids": batch,
			"type":       "market",
		}

		if err := conn.WriteJSON(subscribeMsg); err != nil {
			p.logger.Warnf("Failed to subscribe batch %d-%d: %v", i, end, err)
			continue
		}
		time.Sleep(50 * time.Millisecond) // Rate limit between batches
	}

	p.logger.Info("Subscription complete")
	return nil
}

func (p *PolymarketConnector) readLoop() {
	defer p.conn.Close()

	for {
		_, message, err := p.conn.ReadMessage()
		if err != nil {
			p.logger.Error("Read error:", err)
			return
		}
		p.processMessage(message)
	}
}

func (p *PolymarketConnector) processMessage(data []byte) {
	// Polymarket sends messages as arrays of book updates
	var updates []map[string]interface{}
	if err := json.Unmarshal(data, &updates); err != nil {
		// Try single object format as fallback
		var single map[string]interface{}
		if err := json.Unmarshal(data, &single); err != nil {
			return
		}
		updates = []map[string]interface{}{single}
	}

	for _, msg := range updates {
		// Extract Asset ID
		assetID, _ := msg["asset_id"].(string)
		if assetID == "" {
			continue
		}

		var price float64

		// Check for price field directly (price_change events)
		if priceStr, ok := msg["price"].(string); ok {
			fmt.Sscanf(priceStr, "%f", &price)
		} else if priceFloat, ok := msg["price"].(float64); ok {
			price = priceFloat
		}

		// If no direct price, calculate mid from orderbook
		if price == 0 {
			bids, _ := msg["bids"].([]interface{})
			asks, _ := msg["asks"].([]interface{})

			var bestBid, bestAsk float64

			// Find highest bid
			for _, b := range bids {
				if bid, ok := b.(map[string]interface{}); ok {
					if priceStr, ok := bid["price"].(string); ok {
						var p float64
						fmt.Sscanf(priceStr, "%f", &p)
						if p > bestBid {
							bestBid = p
						}
					}
				}
			}

			// Find lowest ask
			bestAsk = 999.0
			for _, a := range asks {
				if ask, ok := a.(map[string]interface{}); ok {
					if priceStr, ok := ask["price"].(string); ok {
						var p float64
						fmt.Sscanf(priceStr, "%f", &p)
						if p < bestAsk {
							bestAsk = p
						}
					}
				}
			}
			if bestAsk == 999.0 {
				bestAsk = 0
			}

			if bestBid > 0 && bestAsk > 0 {
				price = (bestBid + bestAsk) / 2
			} else if bestBid > 0 {
				price = bestBid
			} else if bestAsk > 0 {
				price = bestAsk
			}
		}

		if price == 0 {
			continue
		}

		// Parse timestamp if available
		var sourceTS int64
		if tsStr, ok := msg["timestamp"].(string); ok {
			fmt.Sscanf(tsStr, "%d", &sourceTS)
		} else {
			sourceTS = time.Now().UnixMilli()
		}

		// Deduplication
		if lastPrice, ok := p.lastPrices.Load(assetID); ok {
			if lastPrice.(float64) == price {
				continue
			}
		}
		p.lastPrices.Store(assetID, price)

		// Send to Channel
		p.msgChan <- models.Tick{
			Source:          "POLYMARKET",
			ContractID:      assetID,
			Price:           price,
			TimestampSource: sourceTS,
			TimestampIngest: time.Now().UnixMilli(),
		}
	}
}