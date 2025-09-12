package main

import (
	"fmt"
	"log"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"google.golang.org/protobuf/proto"

	pb "gomon/pb"

	"strconv"

	"gomon/kafka"
)

func logGoroutineInfo() string {
	buf := make([]byte, 1024)
	// Capture the stack trace of the current goroutine
	n := runtime.Stack(buf, false)
	// Filter and extract only the goroutine info
	stackTrace := string(buf[:n])
	lines := strings.Split(stackTrace, "\n")

	// Just return the first line, which includes the goroutine ID and state
	if len(lines) > 0 {
		return lines[0]
	}
	return ""
}

func collectCPU(wg *sync.WaitGroup, metrics *pb.Metric) {
	defer wg.Done()
	log.Printf("%s: Collect CPU stats...", logGoroutineInfo())
	cpuUsage, err := cpu.Percent(time.Second, false)
	if err != nil {
		log.Println("Error getting CPU usage:", err)
		return
	}

	if len(cpuUsage) > 0 {
		metrics.CpuUsagePercent = float32(cpuUsage[0])
	}

	log.Printf("%s: CPU Usage: %.2f%%\n", logGoroutineInfo(), cpuUsage[0])
}

func collectMemory(wg *sync.WaitGroup, metrics *pb.Metric) {
	defer wg.Done()
	log.Printf("%s: Collect Memory stats...", logGoroutineInfo())
	vMem, err := mem.VirtualMemory()
	if err != nil {
		log.Println("Error getting memory usage:", err)
		return
	}

	// Convert to GB (1024 * 1024 * 1024)
	totalVm := vMem.Total / (1 << 30)
	usedVm := vMem.Used / (1 << 30)
	// Convert to Mb (1024 * 1024)
	freeVm := vMem.Free / (1 << 20)
	buffers := vMem.Buffers
	cached := vMem.Cached
	swapTotal := vMem.SwapTotal
	swapUsed := vMem.SwapCached
	swapFree := vMem.SwapFree

	metrics.MemoryTotalGb = totalVm
	metrics.MemoryUsedPercent = float32(usedVm)
	metrics.MemoryUsedGb = usedVm
	metrics.MemoryFreeGb = freeVm

	log.Printf("%s: Memory Usage: %.2f%% (Total: %v Gb, Used: %v Gb, Free: %v Mb, Buffers: %v, Cached: %v),"+
		"Swap Usage: SwapTotal: %v, SwapUsed: %v, SwapFree: %v\n", logGoroutineInfo(),
		vMem.UsedPercent, totalVm, usedVm, freeVm, buffers, cached,
		swapTotal, swapUsed, swapFree)
}

func collectDisk(wg *sync.WaitGroup, metric *pb.Metric) {
	defer wg.Done()
	log.Printf("%s: Collect Disk stats...", logGoroutineInfo())
	partitions, err := disk.Partitions(false)
	if err != nil {
		log.Printf("Error fetching disk partitions: %v\n", err)
		return
	}

	for _, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			log.Printf("Error fetching disk usage: %v\n", err)
			continue
		}

		diskTotal := usage.Total / (1 << 30)
		diskUsage := usage.Used / (1 << 30)

		metric.DiskStats = append(metric.DiskStats, &pb.DiskUsage{
			Mountpoint:  partition.Mountpoint,
			UsedPercent: float32(usage.UsedPercent),
			TotalGb:     usage.Total,
			UsedGb:      usage.Used,
		})

		log.Printf("%s: Disk Usage on %v: %.2f%% (Total: %v Gb, Used: %v Gb)\n", logGoroutineInfo(),
			partition.Mountpoint, usage.UsedPercent, diskTotal, diskUsage)
	}

}

func collectNet(wg *sync.WaitGroup, metric *pb.Metric) {
	defer wg.Done()
	log.Printf("%s: Collect Network stats...", logGoroutineInfo())
	// Get initial network stats
	prevCounters, err := net.IOCounters(false)
	if err != nil || len(prevCounters) == 0 {
		log.Println("Error fetching initial network stats:", err)
		return
	}

	// Wait for the interval duration (e.g., 10 seconds)
	time.Sleep(20 * time.Second)

	// Get network stats after the interval
	currCounters, err := net.IOCounters(false)
	if err != nil || len(currCounters) == 0 {
		log.Println("Error fetching current network stats:", err)
		return
	}

	// Calculate delta (difference)
	for i, curr := range currCounters {
		prev := prevCounters[i]
		bytesSentDelta := curr.BytesSent - prev.BytesSent
		bytesRecvDelta := curr.BytesRecv - prev.BytesRecv

		metric.NetStats = append(metric.NetStats, &pb.NetworkUsage{
			InterfaceName: curr.Name,
			BytesSent:     curr.BytesSent,
			BytesReceived: curr.BytesRecv,
		})

		log.Printf("%s: Interface: %s\n", logGoroutineInfo(), curr.Name)
		log.Printf("%s: Sent: %.2f Bytes, Received: %.2f Bytes\n", logGoroutineInfo(),
			float64(bytesSentDelta), float64(bytesRecvDelta))

	}
}

func initLogger() *log.Logger {
	// First create stdout logger for debugging
	bootstrapLog := log.New(os.Stdout, "[INIT] ", log.LstdFlags|log.Lshortfile)
	bootstrapLog.Println("Logger initialization started")

	logDir := "/var/log"
	logFile := filepath.Join(logDir, "agent.log")

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		bootstrapLog.Fatalf("Failed to open log file: %v", err)
	}

	if _, err := os.Stat(logFile); err != nil {
		bootstrapLog.Fatalf("Log file verification failed: %v", err)
	}
	bootstrapLog.Printf("Successfully initialized file logging to %s", logFile)

	return log.New(file, "", log.LstdFlags|log.Lshortfile)
}

func main() {
	logger := initLogger()
	defer func() {
		if f, ok := logger.Writer().(*os.File); ok {
			f.Close()
		}
	}()

	logger.Println("MAIN STARTED")

	// Read Kafka env variables
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		logger.Fatal("KAFKA_BROKERS environment variable is not set")
	}
	kafkaTopic := os.Getenv("KAFKA_TOPIC")
	if kafkaTopic == "" {
		logger.Fatal("KAFKA_TOPIC environment variable is not set")
	}
	logger.Printf("Kafka config - Brokers: %s, Topic: %s", kafkaBrokers, kafkaTopic)

	producer := kafka.NewKafkaProducer(kafkaBrokers, kafkaTopic)
	defer producer.Close()

	i := 0
	sleepInSeconds := 20
	for {

		//Generate CorrelationID
		correlationID := generateCorrelationID()

		// Time to start sending metric
		traceStartTime := time.Now().UTC()

		metric := &pb.Metric{
			Timestamp:      strconv.FormatInt(time.Now().Unix(), 10),
			CorrelationId:  correlationID,
			TraceStartTime: traceStartTime.Format(time.RFC3339Nano),
		}
		i++

		var wg sync.WaitGroup
		wg.Add(4)
		go collectCPU(&wg, metric)
		go collectMemory(&wg, metric)
		go collectDisk(&wg, metric)
		go collectNet(&wg, metric)
		wg.Wait()

		data, err := proto.Marshal(metric)
		if err != nil {
			logger.Printf("ERROR: Failed to marshal metric (Iteration %d): %v", i, err)
			continue
		}

		// Log the actual metric data being sent
		logger.Printf("Sending to Kafka (Iteration %d):\n%s", i, formatMetricForLog(metric))

		kafkaPublishStart := time.Now().UTC()
		metric.KafkaPublishTime = kafkaPublishStart.Format(time.RFC3339Nano)

		if err := producer.SendMessage(data); err != nil {
			logger.Printf("ERROR: Failed to send message (Iteration %d): %v", i, err)
		} else {
			kafkaLatency := time.Since(kafkaPublishStart)
			logger.Printf("Agent vs Kafka publish latency: %v (CorrelationID: %s)",
				kafkaLatency, correlationID)
		}

		logger.Printf("INFO: Cycle completed (Iteration %d, Sleep: %ds)", i, sleepInSeconds)
		time.Sleep(time.Duration(sleepInSeconds) * time.Second)
	}
}

// Helper function to format metric for logging
func formatMetricForLog(m *pb.Metric) string {
	var builder strings.Builder

	// Basic info with hostname validation
	hostname := m.Hostname
	if hostname == "" {
		hostname = "[unknown-host]"
	}
	builder.WriteString(fmt.Sprintf("Host: %s | Timestamp: %s\n",
		hostname, m.Timestamp))

	// CPU
	builder.WriteString(fmt.Sprintf("CPU: %.2f%%\n", m.CpuUsagePercent))

	// Memory - convert GB to more readable format
	builder.WriteString(fmt.Sprintf(
		"Memory: %.2f%% used (%.2fGB/%.2fGB free)\n",
		m.MemoryUsedPercent,
		float64(m.MemoryUsedGb),
		float64(m.MemoryTotalGb),
	))

	// Disk - filter out system mounts and format sizes properly
	if len(m.DiskStats) > 0 {
		builder.WriteString("Disks:\n")
		for _, disk := range m.DiskStats {
			mountPoint := disk.GetMountpoint()
			// Skip system mounts in logs
			if strings.HasPrefix(mountPoint, "/etc/") ||
				strings.HasPrefix(mountPoint, "/dev/") {
				continue
			}

			// Convert bytes to GB (assuming the proto uses bytes)
			totalGB := float64(disk.GetTotalGb()) / 1024 / 1024 / 1024
			usedGB := float64(disk.GetUsedGb()) / 1024 / 1024 / 1024

			builder.WriteString(fmt.Sprintf(
				"  %s: %.2f%% used (%.2fGB/%.2fGB)\n",
				mountPoint,
				disk.GetUsedPercent(),
				usedGB,
				totalGB,
			))
		}
	}

	// Network - filter aggregate interfaces
	if len(m.NetStats) > 0 {
		builder.WriteString("Network:\n")
		for _, net := range m.NetStats {
			ifName := net.GetInterfaceName()
			if ifName == "all" || ifName == "" {
				continue
			}
			builder.WriteString(fmt.Sprintf(
				"  %s: Tx %.2fMB, Rx %.2fMB\n",
				ifName,
				float64(net.GetBytesReceived()),
				float64(net.GetBytesSent()),
			))
		}
	}

	return builder.String()
}

func generateCorrelationID() string {
	return uuid.New().String()
}
