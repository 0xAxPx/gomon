//go:build integration

package integration

import (
	"context"
	"fmt"
	"gomon/kafka"
	"gomon/pb"
	"gomon/testutils"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	kfk "github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

// Send Metric and Check it is parsed by Aggregator
func TestE2E(t *testing.T) {
	kafkaBrokers, err := kafka.GetKafkaBrokers()
	if err != nil {
		t.Skip("KAFKA_BROKERS not set, skipping test")
	}
	kafkaTopic, err := kafka.GetKafkaTopic()
	if err != nil {
		t.Skip("KAFKA_TOPIC not set, skipping test")
	}

	// Kafka Producer and Consumer
	kafkaProducer := kafka.NewKafkaProducer(kafkaBrokers, kafkaTopic)
	defer kafkaProducer.Close()

	kafkaReader := kfk.NewReader(kfk.ReaderConfig{
		Brokers:     strings.Split(kafkaBrokers, ","),
		GroupID:     fmt.Sprintf("e2e-test-%d", time.Now().Unix()),
		Topic:       kafkaTopic,
		StartOffset: kfk.FirstOffset,
		MaxWait:     1 * time.Second,
	})
	defer kafkaReader.Close()

	// Create test metric
	metric := testutils.CreateMetric()
	testCorrelationID := metric.CorrelationId

	metricProto, err := testutils.SerializeMetric(metric)
	if err != nil {
		t.Fatalf("Serialization of metric struct failed due to %v", err)
	}

	// Send message to Kafka
	err = kafkaProducer.SendMessage(metricProto)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Read messages until we find ours (with timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for {
		msg, err := kafkaReader.ReadMessage(ctx)
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		deserialized, err := testutils.DeserializeToMetric(msg.Value)
		if err != nil {
			continue
		}

		// Check if this is OUR message
		if deserialized.CorrelationId == testCorrelationID {
			// Found it! Now verify
			if !proto.Equal(deserialized, metric) {
				t.Logf("Original: %+v", metric)
				t.Logf("Deserialized: %+v", deserialized)
				t.Errorf("Metrics don't match")
			}
			return // Test complete
		}
	}
}

func TestE2EEmptyMetric(t *testing.T) {
	kafkaBrokers, err := kafka.GetKafkaBrokers()
	if err != nil {
		t.Skip("KAFKA_BROKERS not set, skipping test")
	}
	kafkaTopic, err := kafka.GetKafkaTopic()
	if err != nil {
		t.Skip("KAFKA_TOPIC not set, skipping test")
	}

	kafkaProducer := kafka.NewKafkaProducer(kafkaBrokers, kafkaTopic)
	defer kafkaProducer.Close()

	kafkaReader := kfk.NewReader(kfk.ReaderConfig{
		Brokers:     strings.Split(kafkaBrokers, ","),
		GroupID:     fmt.Sprintf("e2e-test-%d", time.Now().Unix()),
		Topic:       kafkaTopic,
		StartOffset: kfk.FirstOffset,
		MaxWait:     1 * time.Second,
	})
	defer kafkaReader.Close()

	metric := &pb.Metric{
		Timestamp:      strconv.FormatInt(time.Now().Unix(), 10),
		CorrelationId:  uuid.New().String(),
		TraceStartTime: time.Now().UTC().Format(time.RFC3339Nano),
		// All other fields zero/empty
	}
	testCorrelationID := metric.CorrelationId

	metricProto, err := testutils.SerializeMetric(metric)
	if err != nil {
		t.Fatalf("Serialization failed: %v", err)
	}

	err = kafkaProducer.SendMessage(metricProto)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for {
		msg, err := kafkaReader.ReadMessage(ctx)
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		deserialized, err := testutils.DeserializeToMetric(msg.Value)
		if err != nil {
			continue
		}

		if deserialized.CorrelationId == testCorrelationID {
			if !proto.Equal(deserialized, metric) {
				t.Logf("Original: %+v", metric)
				t.Logf("Deserialized: %+v", deserialized)
				t.Errorf("Metrics don't match")
			}
			return
		}
	}
}

func TestE2EMultipleMessages(t *testing.T) {
	kafkaBrokers, err := kafka.GetKafkaBrokers()
	if err != nil {
		t.Skip("KAFKA_BROKERS not set, skipping test")
	}
	kafkaTopic, err := kafka.GetKafkaTopic()
	if err != nil {
		t.Skip("KAFKA_TOPIC not set, skipping test")
	}

	kafkaProducer := kafka.NewKafkaProducer(kafkaBrokers, kafkaTopic)
	defer kafkaProducer.Close()

	kafkaReader := kfk.NewReader(kfk.ReaderConfig{
		Brokers:     strings.Split(kafkaBrokers, ","),
		GroupID:     fmt.Sprintf("e2e-test-%d", time.Now().Unix()),
		Topic:       kafkaTopic,
		StartOffset: kfk.FirstOffset,
		MaxWait:     1 * time.Second,
	})
	defer kafkaReader.Close()

	// Send 3 messages rapidly
	metrics := []*pb.Metric{
		testutils.CreateMetric(),
		testutils.CreateMetric(),
		testutils.CreateMetric(),
	}

	correlationIDs := make(map[string]bool)
	for _, m := range metrics {
		correlationIDs[m.CorrelationId] = true
		data, _ := testutils.SerializeMetric(m)
		kafkaProducer.SendMessage(data)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Verify all 3 received
	foundCount := 0
	for foundCount < 3 {
		msg, err := kafkaReader.ReadMessage(ctx)
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		deserialized, err := testutils.DeserializeToMetric(msg.Value)
		if err != nil {
			continue
		}

		if correlationIDs[deserialized.CorrelationId] {
			foundCount++
			t.Logf("Found message %d/3: %s", foundCount, deserialized.CorrelationId)
		}
	}

	if foundCount != 3 {
		t.Errorf("Expected 3 messages, found %d", foundCount)
	}
}
