package main

import (
	"fmt"
	"log"
	"net/http"
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

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-lib/metrics"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func startMetricServer(port string) {
	http.Handle("/metrics", promhttp.Handler())

	go func() {
		log.Printf("Starting metrics server on :%s", port)
		log.Printf("Metrics available at: http://localhost:%s/metrics", port)

		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("Failed to start metrics server: %v", err)
		}

	}()
}

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

func collectCPU(wg *sync.WaitGroup, metrics *pb.Metric, parentSpan opentracing.Span) {
	defer wg.Done()

	// Span for CPU
	cpuSpan := opentracing.StartSpan("collect-cpu", opentracing.ChildOf(parentSpan.Context()))
	defer cpuSpan.Finish()

	log.Printf("%s: Collect CPU stats...", logGoroutineInfo())
	cpuUsage, err := cpu.Percent(time.Second, false)
	if err != nil {
		log.Println("Error getting CPU usage:", err)
		cpuSpan.SetTag("error", true)
		return
	}

	if len(cpuUsage) > 0 {
		metrics.CpuUsagePercent = float32(cpuUsage[0])
		cpuSpan.SetTag("cpu_usage_percent", cpuUsage[0])
	}

	log.Printf("%s: CPU Usage: %.2f%%\n", logGoroutineInfo(), cpuUsage[0])
}

func collectMemory(wg *sync.WaitGroup, metrics *pb.Metric, parentSpan opentracing.Span) {
	defer wg.Done()

	// Span for memory
	memSpan := opentracing.StartSpan("collect-memory", opentracing.ChildOf(parentSpan.Context()))
	defer memSpan.Finish()

	log.Printf("%s: Collect Memory stats...", logGoroutineInfo())
	vMem, err := mem.VirtualMemory()
	if err != nil {
		log.Println("Error getting memory usage:", err)
		memSpan.SetTag("error", true)
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

	memSpan.SetTag("memory_used_percent", vMem.UsedPercent)
	memSpan.SetTag("memory_total_gb", totalVm)

	log.Printf("%s: Memory Usage: %.2f%% (Total: %v Gb, Used: %v Gb, Free: %v Mb, Buffers: %v, Cached: %v),"+
		"Swap Usage: SwapTotal: %v, SwapUsed: %v, SwapFree: %v\n", logGoroutineInfo(),
		vMem.UsedPercent, totalVm, usedVm, freeVm, buffers, cached,
		swapTotal, swapUsed, swapFree)
}

func collectDisk(wg *sync.WaitGroup, metric *pb.Metric, parentSpan opentracing.Span) {
	defer wg.Done()

	// Span for disk stats
	diskSpan := opentracing.StartSpan("collect-disk", opentracing.ChildOf(parentSpan.Context()))
	defer diskSpan.Finish()

	log.Printf("%s: Collect Disk stats...", logGoroutineInfo())
	partitions, err := disk.Partitions(false)
	if err != nil {
		log.Printf("Error fetching disk partitions: %v\n", err)
		diskSpan.SetTag("error", true)
		return
	}

	var totalDiskSpaceGB uint64
	var totalDiskUsedGB uint64
	partitionCount := 0

	for _, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			log.Printf("Error fetching disk usage: %v\n", err)
			continue
		}

		diskTotal := usage.Total / (1 << 30)
		diskUsage := usage.Used / (1 << 30)

		totalDiskSpaceGB += uint64(diskTotal)
		totalDiskUsedGB += uint64(diskUsage)
		partitionCount++

		metric.DiskStats = append(metric.DiskStats, &pb.DiskUsage{
			Mountpoint:  partition.Mountpoint,
			UsedPercent: float32(usage.UsedPercent),
			TotalGb:     usage.Total,
			UsedGb:      usage.Used,
		})

		diskSpan.SetTag("partitions_processed", partitionCount)
		diskSpan.SetTag("total_disk_space_gb", totalDiskSpaceGB)
		diskSpan.SetTag("total_disk_used_gb", totalDiskUsedGB)
		if totalDiskSpaceGB > 0 {
			diskUsagePercent := float64(totalDiskUsedGB) / float64(totalDiskSpaceGB) * 100
			diskSpan.SetTag("total_disk_used_percent", diskUsagePercent)
		}
	}

}

func collectNet(wg *sync.WaitGroup, metric *pb.Metric, parentSpan opentracing.Span) {
	defer wg.Done()

	// Span for net stats
	netSpan := opentracing.StartSpan("collect-network", opentracing.ChildOf(parentSpan.Context()))
	defer netSpan.Finish()

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

	netSpan.SetTag("interfaces_processed", len(metric.NetStats))
}

// Jaeger
func initJaeger() (opentracing.Tracer, func(), error) {
	cfg := jaegercfg.Configuration{
		ServiceName: "gomon-agent",
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1, // Sample 100% of traces for development
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:          true,                             // Enable span logging for debugging
			CollectorEndpoint: "http://jaeger:14268/api/traces", // Jaeger agent HTTP endpoint
		},
	}

	jLogger := jaeger.StdLogger
	jMetricsFactory := metrics.NullFactory

	tracer, closer, err := cfg.NewTracer(
		jaegercfg.Logger(jLogger),
		jaegercfg.Metrics(jMetricsFactory),
	)

	if err != nil {
		return nil, nil, fmt.Errorf("cannot initialize jaeger tracer for agent service: %v", err)
	}

	// Set as global tracer
	opentracing.SetGlobalTracer(tracer)

	return tracer, func() { closer.Close() }, nil
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

	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "2112"
	}

	startMetricServer(metricsPort)

	// init jaeger
	tracer, closer, err := initJaeger()
	if err != nil {
		logger.Fatalf("Failed to initialize Jaeger tracer: %v", err)
	}
	defer closer()

	// Read Kafka env variables
	kafkaBrokers, err := kafka.GetKafkaBrokers()
	if err != nil {
		logger.Fatal(err)
	}
	kafkaTopic, err := kafka.GetKafkaTopic()
	if err != nil {
		logger.Fatal(err)
	}

	logger.Printf("Kafka config - Brokers: %s, Topic: %s", kafkaBrokers, kafkaTopic)

	producer := kafka.NewKafkaProducer(kafkaBrokers, kafkaTopic)
	defer producer.Close()

	i := 0
	sleepInSeconds := 20
	for {

		//Generate CorrelationID
		correlationID := generateCorrelationID()

		rootSpan := tracer.StartSpan("gomon-metrics-collection")
		rootSpan.SetTag("correlation_id", correlationID)
		rootSpan.SetTag("iteration", i+1)

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
		go collectCPU(&wg, metric, rootSpan)
		go collectMemory(&wg, metric, rootSpan)
		go collectDisk(&wg, metric, rootSpan)
		go collectNet(&wg, metric, rootSpan)
		wg.Wait()

		data, err := proto.Marshal(metric)
		if err != nil {
			logger.Printf("ERROR: Failed to marshal metric (Iteration %d): %v", i, err)
			rootSpan.SetTag("error", true)
			rootSpan.Finish()
			continue
		}

		// Log the actual metric data being sent
		logger.Printf("Sending to Kafka (Iteration %d):\n%s", i, formatMetricForLog(metric))

		kafkaSpan := opentracing.StartSpan("kafka-publish", opentracing.ChildOf(rootSpan.Context()))
		kafkaPublishStart := time.Now().UTC()
		metric.KafkaPublishTime = kafkaPublishStart.Format(time.RFC3339Nano)

		if err := producer.SendMessage(data); err != nil {
			logger.Printf("ERROR: Failed to send message (Iteration %d): %v", i, err)
			kafkaSpan.SetTag("error", true)
			kafkaSpan.Finish()
			rootSpan.SetTag("error", true)
		} else {
			kafkaLatency := time.Since(kafkaPublishStart)
			logger.Printf("Agent vs Kafka publish latency: %v (CorrelationID: %s)",
				kafkaLatency, correlationID)
			kafkaSpan.SetTag("latency_ms", kafkaLatency.Milliseconds())
			kafkaSpan.SetTag("success", true)
			kafkaSpan.Finish()
		}

		rootSpan.Finish()

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

// snyk test
