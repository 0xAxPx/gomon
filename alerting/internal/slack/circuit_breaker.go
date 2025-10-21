package slack

import (
	"gomon/alerting/internal/config"
	"sync"
	"time"
)

const (
	CLOSED    = 0
	OPEN      = 1
	HALF_OPEN = 2
)

type CircuitBreaker struct {
	state           int
	failureCount    int
	lastFailureTime time.Time
	config          config.CircuitBreakerConfig
	mutex           sync.RWMutex
}

func NewCircuitBreaker(configCB config.CircuitBreakerConfig) *CircuitBreaker {

	return &CircuitBreaker{
		state:           CLOSED,
		failureCount:    0,
		lastFailureTime: time.Time{},
		config:          configCB,
	}

}

// 1. Get current state (thread-safe read)
func (cb *CircuitBreaker) getState() int {
	cb.mutex.RLock() // multiple goroutines can read at once
	defer cb.mutex.RUnlock()
	return cb.state
}

// 2. Check if we can execute request
func (cb *CircuitBreaker) canExecute() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case CLOSED:
		return true
	case OPEN:
		timeOutDuration := time.Duration(cb.config.TimeoutDuration) * time.Second
		if time.Since(cb.lastFailureTime) > timeOutDuration {
			return true
		}
		return false
	case HALF_OPEN:
		return true
	default:
		return false
	}

}

// 3. Record failure
func (cb *CircuitBreaker) recordFailure() {
	cb.mutex.Lock() // Exclusive access to write
	defer cb.mutex.Unlock()
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.state == CLOSED && cb.failureCount >= cb.config.FailureThreshold {
		cb.state = OPEN
	}

	if cb.state == HALF_OPEN {
		cb.state = OPEN
		cb.failureCount = cb.config.FailureThreshold
	}

}

// 4. Record success
func (cb *CircuitBreaker) recordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount = 0

	if cb.state == HALF_OPEN {
		cb.state = CLOSED
	}

}

func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.failureCount
}
