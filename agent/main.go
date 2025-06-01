package main

import (
	"log"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

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
	time.Sleep(1 * time.Second)

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
	t := 10
	for {
		metric := &pb.Metric{
			Timestamp: strconv.FormatInt(time.Now().Unix(), 10),
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
			log.Fatalf("Failed to marshal proto: %v", err)
		}
		log.Printf("Serialized Metric: %v", data)

		if err := producer.SendMessage(data); err != nil {
			log.Printf("Error sending message: %v", err)
		}

		log.Printf("Finished collecting stats with iterator: %d, waiting %dsec", i, t)
		time.Sleep(time.Duration(t) * time.Second)
	}

}
