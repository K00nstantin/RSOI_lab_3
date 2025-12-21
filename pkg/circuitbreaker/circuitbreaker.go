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
	failureCount    int
	timeout         time.Duration
	lastFailureTime time.Time
	state           State
	mu              sync.RWMutex
}

func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures: maxFailures,
		timeout:     timeout,
		state:       StateClosed,
	}
}

func (cb *CircuitBreaker) Execute(fn func() error, fallback func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateOpen {
		if time.Since(cb.lastFailureTime) >= cb.timeout {
			cb.state = StateHalfOpen
			cb.failureCount = 0
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
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		if cb.failureCount >= cb.maxFailures || cb.state == StateHalfOpen {
			cb.state = StateOpen
		}
		return err
	}

	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.failureCount = 0
	} else if cb.state == StateClosed {
		cb.failureCount = 0
	}

	return nil
}
