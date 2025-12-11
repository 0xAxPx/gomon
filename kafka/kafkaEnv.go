package kafka

import (
	"fmt"
	"os"
)

func GetKafkaBrokers() (string, error) {
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		return "", fmt.Errorf("KAFKA_BROKERS not set")
	}
	return kafkaBrokers, nil
}

func GetKafkaTopic() (string, error) {
	kafkaTopic := os.Getenv("KAFKA_TOPIC")
	if kafkaTopic == "" {
		return "", fmt.Errorf("KAFKA_TOPIC not set")
	}
	return kafkaTopic, nil

}
