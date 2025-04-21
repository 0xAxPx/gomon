package kafka

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaProducer struct {
	Writer *kafka.Writer
}

func NewKafkaProducer(brokers string, topic string) *KafkaProducer {
	// Split the brokers string into a slice of broker addresses
	brokerList := strings.Split(brokers, ",")

	log.Printf("Creating Kafka producer with brokers %v and topic %s", brokerList, topic)
	return &KafkaProducer{
		Writer: kafka.NewWriter(kafka.WriterConfig{
			Brokers:  brokerList, // Pass the slice of broker addresses
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		}),
	}
}

func (kp *KafkaProducer) SendMessage(data []byte) error {
	msg := kafka.Message{
		Key:   []byte(time.Now().String()), // optional key
		Value: data,
	}
	err := kp.Writer.WriteMessages(context.Background(), msg)
	if err != nil {
		log.Printf("Failed to write message to Kafka: %v", err)
		return err
	}
	log.Println("Message sent to Kafka...")
	return nil
}

func (kp *KafkaProducer) Close() {
	if err := kp.Writer.Close(); err != nil {
		log.Printf("Failed to close Kafka writer: %v", err)
	}
}
