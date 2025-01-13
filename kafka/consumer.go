package kafka

import (
	"context"
	"log"

	"github.com/segmentio/kafka-go"
)

type KafkaConsumer struct {
	Reader *kafka.Reader
}

func NewKafkaConsumer(broker string, topic string, groupID string) *KafkaConsumer {
	return &KafkaConsumer{
		Reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  []string{broker},
			GroupID:  groupID,
			Topic:    topic,
			MinBytes: 10e3, // 10KB
			MaxBytes: 10e6, // 10MB
		}),
	}
}

func (kc *KafkaConsumer) ConsumeMessages() {
	for {
		msg, err := kc.Reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("Failed to read message from Kafka: %v", err)
			break
		}
		log.Printf("Received message: Key: %s, Value: %s", string(msg.Key), string(msg.Value))
	}
}

func (kc *KafkaConsumer) Close() {
	if err := kc.Reader.Close(); err != nil {
		log.Printf("Failed to close Kafka reader: %v", err)
	}
}
