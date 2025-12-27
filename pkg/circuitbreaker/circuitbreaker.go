package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	maxFailures     int
	window          time.Duration
	failures        []time.Time
	timeout         time.Duration
	lastFailureTime time.Time
	state           State
	mu              sync.RWMutex
}

func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	return NewCircuitBreakerWithWindow(maxFailures, timeout, 60*time.Second)
}

func NewCircuitBreakerWithWindow(maxFailures int, timeout time.Duration, window time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures: maxFailures,
		window:      window,
		timeout:     timeout,
		state:       StateClosed,
		failures:    make([]time.Time, 0),
	}
}

func (cb *CircuitBreaker) Execute(fn func() error, fallback func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateOpen {
		if time.Since(cb.lastFailureTime) >= cb.timeout {
			cb.state = StateHalfOpen
			cb.failures = cb.failures[:0]
		} else {
			cb.mu.Unlock()
			if fallback != nil {
				err := fallback()
				cb.mu.Lock()
				return err
			}
			cb.mu.Lock()
			return errors.New("circuit breaker is open")
		}
	}

	err := fn()

	if err != nil {
		now := time.Now()
		cb.lastFailureTime = now
		cb.failures = append(cb.failures, now)
		cb.cleanOldFailures(now)

		if len(cb.failures) > cb.maxFailures || cb.state == StateHalfOpen {
			cb.state = StateOpen
		}
		return err
	}

	cb.cleanOldFailures(time.Now())

	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.failures = cb.failures[:0]
	}

	return nil
}

func (cb *CircuitBreaker) cleanOldFailures(now time.Time) {
	cutoff := now.Add(-cb.window)
	validStart := 0
	for i := len(cb.failures) - 1; i >= 0; i-- {
		if cb.failures[i].After(cutoff) {
			validStart = i
			break
		}
	}
	if validStart > 0 {
		cb.failures = cb.failures[validStart:]
	}
}

func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}
