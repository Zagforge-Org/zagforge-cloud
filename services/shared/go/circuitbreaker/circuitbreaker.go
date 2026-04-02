package circuitbreaker

import (
	"time"

	"github.com/sony/gobreaker/v2"
	"go.uber.org/zap"
)

// Breaker wraps gobreaker to provide type-safe execution.
// Circuit state is tracked purely on errors — return types are handled
// via the generic Execute function and the Run method.
type Breaker struct {
	cb *gobreaker.CircuitBreaker[struct{}]
}

// New creates a Breaker with sensible defaults.
//
// The breaker opens after 5 consecutive failures, stays open for 10 seconds,
// then enters half-open state (allows 1 probe request).
func New(name string, log *zap.Logger) *Breaker {
	return &Breaker{cb: gobreaker.NewCircuitBreaker[struct{}](settings(name, 1, 60*time.Second, 10*time.Second, 5, log))}
}

// NewWithConfig creates a Breaker with custom settings.
func NewWithConfig(name string, maxRequests uint32, interval, timeout time.Duration, failureThreshold uint32, log *zap.Logger) *Breaker {
	return &Breaker{cb: gobreaker.NewCircuitBreaker[struct{}](settings(name, maxRequests, interval, timeout, failureThreshold, log))}
}

// Run executes fn through the circuit breaker for operations that only return an error.
func (b *Breaker) Run(fn func() error) error {
	_, err := b.cb.Execute(func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

// Execute runs fn through the circuit breaker, preserving the return type T.
func Execute[T any](b *Breaker, fn func() (T, error)) (T, error) {
	var result T
	_, err := b.cb.Execute(func() (struct{}, error) {
		var fnErr error
		result, fnErr = fn()
		return struct{}{}, fnErr
	})
	return result, err
}

func settings(name string, maxRequests uint32, interval, timeout time.Duration, failureThreshold uint32, log *zap.Logger) gobreaker.Settings {
	return gobreaker.Settings{
		Name:        name,
		MaxRequests: maxRequests,
		Interval:    interval,
		Timeout:     timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= failureThreshold
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Warn("circuit breaker state change",
				zap.String("breaker", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	}
}
