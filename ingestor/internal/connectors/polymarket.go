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

	// 2. Subscribe
	for _, id := range tokenIDs {
		subscribeMsg := map[string]interface{}{
			"type":   "subscribe",
			"market": id,
		}

		if err := conn.WriteJSON(subscribeMsg); err != nil {
			continue
		}
		time.Sleep(5 * time.Millisecond) // Rate limit
	}
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
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	// Filter types
	msgType, _ := msg["type"].(string)
	if msgType != "price_update" && msgType != "book_update" && msgType != "trade" {
		return
	}

	// Extract Asset ID
	assetID, _ := msg["asset_id"].(string)
	if assetID == "" {
		tokenID, _ := msg["token_id"].(string)
		assetID = tokenID
	}
	
	if assetID == "" {
		return
	}

	// Extract Price
	price, _ := msg["price"].(float64)
	if price == 0 {
		return 
	}

	// Deduplication
	if lastPrice, ok := p.lastPrices.Load(assetID); ok {
		if lastPrice.(float64) == price {
			return 
		}
	}
	p.lastPrices.Store(assetID, price)

	// Send to Channel
	p.msgChan <- models.Tick{
		Source:          "POLYMARKET",
		ContractID:      assetID,
		Price:           price,
		TimestampSource: time.Now().UnixMilli(),
		TimestampIngest: time.Now().UnixMilli(),
	}
}