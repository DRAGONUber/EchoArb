package retry

import (
	"context"
	"math/rand"
	"sync"
	"time"
	"go.uber.org/zap"
)

type Config struct {
	InitialInterval time.Duration
	MaxInterval     time.Duration
	MaxRetries      int // 0 = infinite
	Multiplier      float64
	Jitter          bool
}

// RetryForever executes a function indefinitely with exponential backoff
func RetryForever(ctx context.Context, cfg Config, logger *zap.SugaredLogger, name string, fn func() error) {
	interval := cfg.InitialInterval
	attempt := 0

	for {
		if ctx.Err() != nil {
			return
		}

		err := fn()
		if err == nil {
			// Reset on success
			interval = cfg.InitialInterval
			attempt = 0
			continue
		}

		logger.Warnf("[%s] Connection failed: %v. Retrying in %v", name, err, interval)

		select {
		case <-time.After(interval):
			// Calculate next interval: min(current * multiplier, max)
			interval = time.Duration(float64(interval) * cfg.Multiplier)
			if interval > cfg.MaxInterval {
				interval = cfg.MaxInterval
			}
			// Add Jitter (±25%)
			if cfg.Jitter {
				jitter := time.Duration(float64(interval) * 0.25 * (2*rand.Float64() - 1))
				interval += jitter
			}
			attempt++
		case <-ctx.Done():
			return
		}
	}
}

// CircuitBreaker state
type CircuitBreaker struct {
	threshold int
	timeout   time.Duration
	failures  int
	state     string // "CLOSED", "OPEN", "HALF_OPEN"
	lastFail  time.Time
	mu        sync.Mutex
}

func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{threshold: threshold, timeout: timeout, state: "CLOSED"}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == "OPEN" {
		if time.Since(cb.lastFail) > cb.timeout {
			cb.state = "HALF_OPEN"
			return true
		}
		return false
	}
	return true
}

func (cb *CircuitBreaker) RecordResult(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if success {
		if cb.state == "HALF_OPEN" {
			cb.state = "CLOSED"
			cb.failures = 0
		} else if cb.state == "CLOSED" {
			cb.failures = 0
		}
	} else {
		cb.failures++
		if cb.failures >= cb.threshold {
			cb.state = "OPEN"
			cb.lastFail = time.Now()
		}
	}
}