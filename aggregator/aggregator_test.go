package main

import (
	"fmt"
	pb "gomon/pb"
	"gomon/testutils"
	"testing"

	"encoding/json"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

// Protobuf deserialization (Kafka → Go struct)
func TestKafkaDeserialization(t *testing.T) {
	fmt.Println("Running Kafka deserialization test")

	// Generate Metric struct
	metric := testutils.CreateMetric()

	// Serialize struct to proto
	metricProto, err := testutils.SerializeMetric(metric)
	if err != nil {
		t.Errorf("Error returned while serializing metric to proto format %v", err)
	}

	// Generate kafka message
	//kafkaMessage := &kafka.Message{
	//	Value: metricProto,
	//}

	// Deserialize
	//var deserialized pb.Metric
	//err = proto.Unmarshal(kafkaMessage.Value, &deserialized)

	deserialized, err := deserializeKafka(metricProto)
	if err != nil {
		t.Errorf("Deserialization failed: %v", err)
	}

	if !proto.Equal(deserialized, metric) {
		t.Errorf("Metrics don't match")
	}

}

func deserializeKafka(metrics []byte) (*pb.Metric, error) {
	// Generate kafka message
	kafkaMessage := &kafka.Message{
		Value: metrics,
	}

	// Deserialize
	var deserialized pb.Metric
	err := proto.Unmarshal(kafkaMessage.Value, &deserialized)
	if err != nil {
		return nil, err
	}

	return &deserialized, nil

}

// VictoriaMetrics serialization (Go struct → VM format)
func TestVMSerialization(t *testing.T) {
	correlationID := testutils.GetCorrelationID()
	hostname, err := testutils.GetHostname()
	if err != nil {
		t.Error("Error while getting hostname!")
	}
	var metricsData []map[string]interface{}
	var metricsProcessed = 0
	metric := testutils.CreateMetric()

	if metric.CpuUsagePercent > 0 {
		data := CreateMetricData("cpu_usage_percent", float64(metric.CpuUsagePercent), metric.Timestamp, correlationID, hostname)
		metricsData = append(metricsData, data)
		metricsProcessed++
	}

	// Memory metric
	if metric.MemoryUsedPercent > 0 {
		data := CreateMetricData("mem_usage_percent", float64(metric.MemoryUsedPercent), metric.Timestamp, correlationID, hostname)
		metricsData = append(metricsData, data)
		metricsProcessed++
	}

	// Disk used GB metric
	if metric.MemoryUsedGb > 0 {
		data := CreateMetricData("dsk_used_gb", float64(metric.MemoryUsedGb), metric.Timestamp, correlationID, hostname)
		metricsData = append(metricsData, data)
		metricsProcessed++
	}

	// Disk stats
	for _, disk := range metric.DiskStats {
		if disk != nil {
			data := CreateMetricData("disk_used_percent", float64(disk.UsedPercent), metric.Timestamp, correlationID, hostname)
			metricsData = append(metricsData, data)
			metricsProcessed++
		}
	}

	// Network stats
	for _, net := range metric.NetStats {
		if net != nil {
			// Bytes received
			data1 := CreateMetricData("int_bytes_recv_mb", float64(net.BytesReceived>>20), metric.Timestamp, correlationID, hostname)
			metricsData = append(metricsData, data1)

			// Bytes sent
			data2 := CreateMetricData("int_bytes_sent_mb", float64(net.BytesSent>>20), metric.Timestamp, correlationID, hostname)
			metricsData = append(metricsData, data2)
			metricsProcessed += 2
		}
	}

	jsonData, err := json.Marshal(metricsData)
	if err != nil {
		t.Errorf("Could not marshal JSON: %v", err)
	}

	// 1. Check it's not empty
	if len(jsonData) == 0 {
		t.Error("JSON data is empty")
	}

	// 2. Verify it's valid JSON by unmarshaling back
	var decoded []map[string]interface{}
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Errorf("Invalid JSON generated: %v", err)
	}

	// 3. Check count matches
	if len(decoded) != metricsProcessed {
		t.Errorf("Expected %d metrics, got %d", metricsProcessed, len(decoded))
	}

	// 4. Check specific field exists
	if decoded[0]["metric"] == nil {
		t.Error("Missing 'metric' field in JSON")
	}
	fmt.Printf("JSON decoded %v", decoded)

}
