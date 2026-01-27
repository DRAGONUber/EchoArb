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