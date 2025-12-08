package testutils

import (
	pb "gomon/pb"
	"strconv"
	"time"

	"os"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

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

func GetCorrelationID() string {
	return uuid.New().String()
}

func GetHostname() (string, error) {
	return os.Hostname()
}
