package main

import (
	pb "gomon/pb"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"fmt"
)

func TestSerialization(t *testing.T) {
	correlationID := uuid.New().String()
	metric := &pb.Metric{}

	//set CPU metrics
	metric.CpuUsagePercent = float32(44)

	//set mem metrics
	metric.MemoryTotalGb = uint64(33)
	metric.MemoryFreeGb = uint64(12)
	metric.MemoryUsedGb = metric.MemoryTotalGb - metric.MemoryFreeGb
	metric.MemoryUsedPercent = float32(metric.MemoryUsedGb) / float32(metric.MemoryTotalGb) * 100

	//set Disc metrics
	metric.DiskStats = append(metric.DiskStats, &pb.DiskUsage{
		Mountpoint:  "/dev/da",
		UsedPercent: float32(33.4),
		TotalGb:     120,
		UsedGb:      45,
	})

	metric.NetStats = append(metric.NetStats, &pb.NetworkUsage{
		InterfaceName: "eth0",
		BytesSent:     283303,
		BytesReceived: 19203302,
	})

	traceStartTime := time.Now().UTC()

	metric.Timestamp = strconv.FormatInt(time.Now().Unix(), 10)
	metric.CorrelationId = correlationID
	metric.TraceStartTime = traceStartTime.Format(time.RFC3339Nano)

	fmt.Printf("Metrics generate %+v", metric)

	// Marshar - > Serialize metrics
	dataInBinary, err := proto.Marshal(metric)
	if err != nil {
		fmt.Println("Failed to serialize metrics!")
		t.Fail()
	}
	fmt.Printf("Metrics in binary format %+v\n", dataInBinary)

	// Unmarshal / Deserialize
	deserialized := &pb.Metric{}
	err = proto.Unmarshal(dataInBinary, deserialized)
	if err != nil {
		fmt.Println("Failed to serialize metrics!")
		t.Fail()
	}

	if deserialized.CpuUsagePercent != metric.CpuUsagePercent {
		t.Errorf("CPU mismatch: got %v, want %v", deserialized.CpuUsagePercent, metric.CpuUsagePercent)
	}
	if deserialized.MemoryTotalGb != metric.MemoryTotalGb {
		t.Errorf("Memory mismatch: got %v, want %v", deserialized.MemoryTotalGb, metric.MemoryTotalGb)
	}
	if deserialized.MemoryFreeGb != metric.MemoryFreeGb {
		t.Errorf("Memory Free mismatch: got %v, want %v", deserialized.MemoryFreeGb, metric.MemoryFreeGb)
	}
	if deserialized.MemoryUsedGb != metric.MemoryUsedGb {
		t.Errorf("Memory Used mismatch: got %v, want %v", deserialized.MemoryUsedGb, metric.MemoryUsedGb)
	}
	if deserialized.MemoryUsedPercent != metric.MemoryUsedPercent {
		t.Errorf("Memory Used Perc mismatch: got %v, want %v", deserialized.MemoryUsedPercent, metric.MemoryUsedPercent)
	}
	if !proto.Equal(deserialized.DiskStats[0], metric.DiskStats[0]) {
		t.Errorf("DiskStats mismatch: got %v, want %v", deserialized.DiskStats[0], metric.DiskStats[0])
	}
	if !proto.Equal(deserialized.NetStats[0], metric.NetStats[0]) {
		t.Errorf("NetStats mismatch: got %v, want %v", deserialized.NetStats[0], metric.NetStats[0])
	}
}
