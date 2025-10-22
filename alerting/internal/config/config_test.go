package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Set config path
	os.Setenv("CONFIG_PATH", "../../configs/prod.yaml")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify circuit breaker config
	if cfg.Slack.CircuitBreaker.FailureThreshold != 5 {
		t.Errorf("Expected FailureThreshold=5, got %d", cfg.Slack.CircuitBreaker.FailureThreshold)
	}

	if cfg.Slack.CircuitBreaker.TimeoutDuration != 60 {
		t.Errorf("Expected TimeoutDuration=60, got %d", cfg.Slack.CircuitBreaker.TimeoutDuration)
	}

	if cfg.Slack.CircuitBreaker.HalfOpenMaxRequests != 3 {
		t.Errorf("Expected HalfOpenMaxRequests=3, got %d", cfg.Slack.CircuitBreaker.HalfOpenMaxRequests)
	}

	t.Logf("✅ Config loaded successfully:")
	t.Logf("   FailureThreshold: %d", cfg.Slack.CircuitBreaker.FailureThreshold)
	t.Logf("   TimeoutDuration: %d", cfg.Slack.CircuitBreaker.TimeoutDuration)
	t.Logf("   HalfOpenMaxRequests: %d", cfg.Slack.CircuitBreaker.HalfOpenMaxRequests)
}
