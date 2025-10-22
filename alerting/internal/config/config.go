package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Http  HttpConfig  `yaml:"http"`
	Db    DbConfig    `yaml:"db"`
	Slack SlackConfig `yaml:"slack"`
}

type HttpConfig struct {
	Port int `yaml:"port"`
}

type DbConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type SlackConfig struct {
	Enabled        bool                 `yaml:"enabled"`
	Channels       map[string]string    `yaml:"channels"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}

type CircuitBreakerConfig struct {
	FailureThreshold    int `yaml:"failure_threshold"`
	TimeoutDuration     int `yaml:"timeout_duration"`
	HalfOpenMaxRequests int `yaml:"half_open_max_requests"`
}

func Load() (Config, error) {
	configPath := os.Getenv("CONFIG_PATH")

	if configPath == "" {
		// Default: try relative path from project root
		configPath = "configs/prod.yaml"

		// If that doesn't exist, try from cmd/alerter
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			configPath = "../../configs/prod.yaml"
		}
	}

	byteYaml, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("could not read %s: %w", configPath, err)
	}

	var config Config
	if err := yaml.Unmarshal(byteYaml, &config); err != nil {
		return Config{}, fmt.Errorf("could not unmarshal config: %w", err)
	}

	return config, nil
}

func GetSlackToken() string {
	return os.Getenv("SLACK_BOT_TOKEN")
}
