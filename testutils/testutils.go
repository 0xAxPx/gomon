package testutils

import (
	pb "gomon/pb"
	"strconv"
	"time"

	"os"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

func CreateMetricWithOptions(opts ...MetricOption) *pb.Metric {
	m := CreateMetric() // Default metric

	for _, opt := range opts {
		opt(m)
	}

	return m
}

type MetricOption func(*pb.Metric)

func WithCPU(cpu float32) MetricOption {
	return func(m *pb.Metric) {
		m.CpuUsagePercent = cpu
	}
}

func WithMemory(total, free uint64) MetricOption {
	return func(m *pb.Metric) {
		m.MemoryTotalGb = total
		m.MemoryFreeGb = free
		m.MemoryUsedGb = total - free
		m.MemoryUsedPercent = float32(total-free) / float32(total) * 100
	}
}

func WithDisks(disks []*pb.DiskUsage) MetricOption {
	return func(m *pb.Metric) {
		m.DiskStats = disks
	}
}

func CreateMetric() *pb.Metric {
	now := time.Now()
	memTotal := uint64(33)
	memFree := uint64(12)
	memUsed := memTotal - memFree

	return &pb.Metric{
		Timestamp:         strconv.FormatInt(now.Unix(), 10),
		CpuUsagePercent:   44,
		MemoryTotalGb:     memTotal,
		MemoryFreeGb:      memFree,
		MemoryUsedGb:      memUsed,
		MemoryUsedPercent: float32(memUsed) / float32(memTotal) * 100,
		DiskStats: []*pb.DiskUsage{{
			Mountpoint:  "/dev/da",
			UsedPercent: 33.4,
			TotalGb:     120,
			UsedGb:      45,
		}},
		NetStats: []*pb.NetworkUsage{{
			InterfaceName: "eth0",
			BytesSent:     283303,
			BytesReceived: 19203302,
		}},
		TraceStartTime: now.UTC().Format(time.RFC3339Nano),
		CorrelationId:  uuid.New().String(),
	}
}

func SerializeMetric(metric *pb.Metric) ([]byte, error) {
	dataInBinary, err := proto.Marshal(metric)
	if err != nil {
		return nil, err
	}
	return dataInBinary, nil
}

func DeserializeToMetric(metrics []byte) (*pb.Metric, error) {
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

func GetCorrelationID() string {
	return uuid.New().String()
}

func GetHostname() (string, error) {
	return os.Hostname()
}
