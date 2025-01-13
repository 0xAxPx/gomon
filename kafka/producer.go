package kafka

import (
	"context"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaProducer struct {
	Writer *kafka.Writer
}

func NewKafkaProducer(broker string, topic string) *KafkaProducer {
	log.Printf("Creating Kafka producer %s with topic %s", broker, topic)
	return &KafkaProducer{
		Writer: kafka.NewWriter(kafka.WriterConfig{
			Brokers:  []string{broker},
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
