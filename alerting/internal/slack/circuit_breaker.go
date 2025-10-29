package slack

import (
	"gomon/alerting/internal/config"
	"gomon/alerting/internal/metrics"
	"log"
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
	metrics         *metrics.Metrics
}

func NewCircuitBreaker(configCB config.CircuitBreakerConfig, metrics *metrics.Metrics) *CircuitBreaker {

	log.Printf("🔧 Circuit breaker initialized: threshold=%d, timeout=%ds, half_open=%d",
		configCB.FailureThreshold,
		configCB.TimeoutDuration,
		configCB.HalfOpenMaxRequests)

	cb := &CircuitBreaker{
		state:           CLOSED,
		failureCount:    0,
		lastFailureTime: time.Time{},
		config:          configCB,
		metrics:         metrics,
	}

	metrics.SetCircuitBreakerState(0)

	return cb

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

	log.Printf("🔍 CB Debug: state=%d, failures=%d, threshold=%d",
		cb.state, cb.failureCount, cb.config.FailureThreshold)

	if cb.state == CLOSED && cb.failureCount >= cb.config.FailureThreshold {
		cb.state = OPEN
		log.Printf("🔴 CIRCUIT BREAKER OPENED! Failures: %d (threshold: %d)",
			cb.failureCount, cb.config.FailureThreshold)
		cb.metrics.SetCircuitBreakerState(1)
	}

	if cb.state == HALF_OPEN {
		cb.state = OPEN
		cb.failureCount = cb.config.FailureThreshold
		log.Printf("🔴 Circuit breaker reopened from HALF_OPEN state")
	}

}

// 4. Record success
func (cb *CircuitBreaker) recordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount = 0

	if cb.state == HALF_OPEN {
		cb.state = CLOSED
		log.Printf("🟢 Circuit breaker CLOSED (recovered)")
		cb.metrics.SetCircuitBreakerState(0)
	}

}

func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.failureCount
}
