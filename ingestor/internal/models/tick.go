// internal/models/tick.go
package models

import (
	"encoding/json"
	"time"
)

// Tick represents a normalized market price update
type Tick struct {
	Source          string  `json:"source" msgpack:"source"`
	ContractID      string  `json:"contract_id" msgpack:"contract_id"`
	Price           float64 `json:"price" msgpack:"price"`
	TimestampSource int64   `json:"ts_source" msgpack:"ts_source"` // Exchange timestamp (ms)
	TimestampIngest int64   `json:"ts_ingest" msgpack:"ts_ingest"` // Our receive time (ms)

	// Price fields
	YesBid    float64 `json:"yes_bid,omitempty" msgpack:"yes_bid,omitempty"`       // Best bid for YES (0-1)
	YesAsk    float64 `json:"yes_ask,omitempty" msgpack:"yes_ask,omitempty"`       // Best ask for YES (0-1)
	NoBid     float64 `json:"no_bid,omitempty" msgpack:"no_bid,omitempty"`         // Best bid for NO (0-1)
	NoAsk     float64 `json:"no_ask,omitempty" msgpack:"no_ask,omitempty"`         // Best ask for NO (0-1)
	LastPrice float64 `json:"last_price,omitempty" msgpack:"last_price,omitempty"` // Last trade price (0-1)

	// Size/Depth fields
	BidSize   float64 `json:"bid_size,omitempty" msgpack:"bid_size,omitempty"`     // Size at best bid
	AskSize   float64 `json:"ask_size,omitempty" msgpack:"ask_size,omitempty"`     // Size at best ask
	TradeSize float64 `json:"trade_size,omitempty" msgpack:"trade_size,omitempty"` // Size of last trade

	// Volume fields
	Volume           int64 `json:"volume,omitempty" msgpack:"volume,omitempty"`                       // Contracts traded
	OpenInterest     int64 `json:"open_interest,omitempty" msgpack:"open_interest,omitempty"`         // Active contracts
	DollarVolume     int64 `json:"dollar_volume,omitempty" msgpack:"dollar_volume,omitempty"`         // Dollars traded (Kalshi)
	DollarOpenInt    int64 `json:"dollar_open_interest,omitempty" msgpack:"dollar_open_interest,omitempty"` // Dollars positioned (Kalshi)

	// Trade info
	TradeSide   string  `json:"trade_side,omitempty" msgpack:"trade_side,omitempty"`     // BUY/SELL or YES/NO
	FeeRateBps  float64 `json:"fee_rate_bps,omitempty" msgpack:"fee_rate_bps,omitempty"` // Fee rate in basis points (Polymarket)

	// Market metadata
	MarketID   string `json:"market_id,omitempty" msgpack:"market_id,omitempty"`     // Market UUID (Kalshi) or Condition ID (Polymarket)
	MarketName string `json:"market_name,omitempty" msgpack:"market_name,omitempty"` // Human-readable name
	EventType  string `json:"event_type,omitempty" msgpack:"event_type,omitempty"`   // Message type that generated this tick
}

// Validate checks if tick data is valid
func (t *Tick) Validate() error {
	if t.Source == "" {
		return ErrMissingSource
	}
	if t.ContractID == "" {
		return ErrMissingContractID
	}
	if t.Price < 0 || t.Price > 1 {
		return ErrInvalidPrice
	}
	return nil
}

// LatencyMS returns the latency in milliseconds from source to ingest
func (t *Tick) LatencyMS() int64 {
	return t.TimestampIngest - t.TimestampSource
}

// ToJSON converts tick to JSON bytes
func (t *Tick) ToJSON() ([]byte, error) {
	return json.Marshal(t)
}

// Errors
var (
	ErrMissingSource     = &TickError{"source is required"}
	ErrMissingContractID = &TickError{"contract_id is required"}
	ErrInvalidPrice      = &TickError{"price must be between 0 and 1"}
)

type TickError struct {
	Message string
}

func (e *TickError) Error() string {
	return e.Message
}

// OrderbookSnapshot represents a full orderbook state
type OrderbookSnapshot struct {
	Ticker      string         `json:"ticker"`
	Timestamp   int64          `json:"timestamp"`
	YesBids     []PriceLevel   `json:"yes_bids"`
	YesAsks     []PriceLevel   `json:"yes_asks"`
	NoBids      []PriceLevel   `json:"no_bids"`
	NoAsks      []PriceLevel   `json:"no_asks"`
}

// PriceLevel represents a single level in the orderbook
type PriceLevel struct {
	Price    int `json:"price"`    // Price in cents
	Quantity int `json:"quantity"` // Quantity available
}

// OrderbookDelta represents an incremental update
type OrderbookDelta struct {
	Ticker    string       `json:"ticker"`
	Timestamp int64        `json:"timestamp"`
	Updates   []BookUpdate `json:"updates"`
}

// BookUpdate represents a single orderbook change
type BookUpdate struct {
	Side     string `json:"side"`     // "yes" or "no"
	Type     string `json:"type"`     // "bid" or "ask"
	Price    int    `json:"price"`
	Quantity int    `json:"quantity"` // 0 means removed
}

// MarketStatus represents the current status of a market
type MarketStatus struct {
	Ticker       string    `json:"ticker"`
	Status       string    `json:"status"` // "open", "closed", "settled"
	LastPrice    int       `json:"last_price"`
	Volume24h    int       `json:"volume_24h"`
	OpenInterest int       `json:"open_interest"`
	UpdatedAt    time.Time `json:"updated_at"`
}