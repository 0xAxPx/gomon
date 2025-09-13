package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gomon/pb"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-lib/metrics"
)

// Jaeger
func initJaeger() (opentracing.Tracer, func(), error) {
	cfg := jaegercfg.Configuration{
		ServiceName: "gomon-aggregator",
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
		return nil, nil, fmt.Errorf("cannot initialize jaeger tracer for aggregator service: %v", err)
	}

	// Set as global tracer
	opentracing.SetGlobalTracer(tracer)

	return tracer, func() { closer.Close() }, nil
}

func initLogger() *log.Logger {
	bootstrapLog := log.New(os.Stdout, "[INIT] ", log.LstdFlags|log.Lshortfile)
	bootstrapLog.Println("Logger initialization started")

	logDir := "/var/log"
	logFile := filepath.Join(logDir, "aggregator.log")

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

func StartAggregator(logger *log.Logger) error {

	// init jaeger
	tracer, closer, err := initJaeger()
	if err != nil {
		logger.Fatalf("Failed to initialize Jaeger tracer: %v", err)
	}
	defer closer()

	// Read Kafka env variable
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		logger.Fatal("KAFKA_BROKERS environment variable is not set")
	}

	// Read Kafka topic from environment variable
	kafkaTopic := os.Getenv("KAFKA_TOPIC")
	if kafkaTopic == "" {
		logger.Fatal("KAFKA_TOPIC environment variable is not set")
	}

	log.Printf("Creating Kafka producer with brokers %v and topic %s", kafkaBrokers, kafkaTopic)
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(kafkaBrokers, ","),
		GroupID: "metrics-group",
		Topic:   kafkaTopic,
	})
	defer reader.Close()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigs:
			logger.Println("Received termination signal, stopping aggregator...")
			return nil
		default:
			kafkaReceiveStart := time.Now().UTC()
			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				logger.Printf("Could not read message: %v", err)
				continue
			}

			err = processAndSendMetrics(msg.Value, logger, kafkaReceiveStart, tracer)
			if err != nil {
				logger.Printf("Error processing message: %v", err)
			}
		}
	}
}

// processAndSendMetrics processes and sends separate metrics to VictoriaMetrics
func processAndSendMetrics(protoData []byte, logger *log.Logger, kafkaReceiveStart time.Time, tracer opentracing.Tracer) error {

	// Create NEW root span for aggregator
	aggregatorRootSpan := tracer.StartSpan("gomon-aggregator-processing")
	defer aggregatorRootSpan.Finish()

	// SPAN 1: kafka-consume (includes unmarshalling)
	kafkaSpan := opentracing.StartSpan("kafka-consume", opentracing.ChildOf(aggregatorRootSpan.Context()))

	var metric pb.Metric
	err := proto.Unmarshal(protoData, &metric)
	if err != nil {
		kafkaSpan.SetTag("error", true)
		kafkaSpan.Finish()
		aggregatorRootSpan.SetTag("error", true)
		return fmt.Errorf("could not unmarshal protobuf data: %v", err)
	}

	correlationID := metric.CorrelationId
	aggregatorRootSpan.SetTag("correlation_id", correlationID)

	kafkaLatency := time.Since(kafkaReceiveStart)
	kafkaSpan.SetTag("kafka_latency_ms", kafkaLatency.Milliseconds())
	kafkaSpan.SetTag("success", true)
	kafkaSpan.Finish()

	logger.Printf("Kafka vs Aggregator latency: %v (CorrelationID: %s)", kafkaLatency, correlationID)

	// SPAN 2: process-metrics (prepare all metric data)
	processSpan := opentracing.StartSpan("process-metrics", opentracing.ChildOf(aggregatorRootSpan.Context()))

	// Prepare all metrics data for VictoriaMetrics
	var metricsData []map[string]interface{}

	// Get hostname once
	hostname, err := os.Hostname()
	if err != nil {
		processSpan.SetTag("error", true)
		processSpan.Finish()
		return fmt.Errorf("error getting hostname: %v", err)
	}

	// Process individual metrics
	metricsProcessed := 0

	// CPU metric
	if metric.CpuUsagePercent > 0 {
		data := createMetricData("cpu_usage_percent", float64(metric.CpuUsagePercent), metric.Timestamp, correlationID, hostname)
		metricsData = append(metricsData, data)
		metricsProcessed++
	}

	// Memory metric
	if metric.MemoryUsedPercent > 0 {
		data := createMetricData("mem_usage_percent", float64(metric.MemoryUsedPercent), metric.Timestamp, correlationID, hostname)
		metricsData = append(metricsData, data)
		metricsProcessed++
	}

	// Disk used GB metric
	if metric.MemoryUsedGb > 0 {
		data := createMetricData("dsk_used_gb", float64(metric.MemoryUsedGb), metric.Timestamp, correlationID, hostname)
		metricsData = append(metricsData, data)
		metricsProcessed++
	}

	// Disk stats
	for _, disk := range metric.DiskStats {
		if disk != nil {
			data := createMetricData("disk_used_percent", float64(disk.UsedPercent), metric.Timestamp, correlationID, hostname)
			metricsData = append(metricsData, data)
			metricsProcessed++
		}
	}

	// Network stats
	for _, net := range metric.NetStats {
		if net != nil {
			// Bytes received
			data1 := createMetricData("int_bytes_recv_mb", float64(net.BytesReceived>>20), metric.Timestamp, correlationID, hostname)
			metricsData = append(metricsData, data1)

			// Bytes sent
			data2 := createMetricData("int_bytes_sent_mb", float64(net.BytesSent>>20), metric.Timestamp, correlationID, hostname)
			metricsData = append(metricsData, data2)
			metricsProcessed += 2
		}
	}

	processSpan.SetTag("metrics_processed", metricsProcessed)
	processSpan.SetTag("success", true)
	processSpan.Finish()

	// SPAN 3: victoria-metrics-publish (all HTTP calls)
	vmSpan := opentracing.StartSpan("victoria-metrics-publish", opentracing.ChildOf(aggregatorRootSpan.Context()))

	successfulSends := 0
	failedSends := 0

	for _, data := range metricsData {
		err := sendToVictoriaMetrics(data, logger)
		if err != nil {
			failedSends++
			logger.Printf("Error sending metric to VictoriaMetrics: %v", err)
		} else {
			successfulSends++
		}
	}

	vmSpan.SetTag("successful_sends", successfulSends)
	vmSpan.SetTag("failed_sends", failedSends)

	if failedSends > 0 {
		vmSpan.SetTag("error", true)
		aggregatorRootSpan.SetTag("error", true)
	} else {
		vmSpan.SetTag("success", true)
	}

	vmSpan.Finish()

	if failedSends > 0 {
		return fmt.Errorf("failed to send %d out of %d metrics to VictoriaMetrics", failedSends, len(metricsData))
	}

	logger.Printf("Successfully processed and sent %d metrics to VictoriaMetrics", successfulSends)
	return nil
}

// Helper function to create metric data structure
func createMetricData(metricName string, value float64, timestampStr string, correlationID string, hostname string) map[string]interface{} {
	timestamp, _ := strconv.ParseInt(timestampStr, 10, 64)

	return map[string]interface{}{
		"metric": map[string]string{
			"__name__":       metricName,
			"job":            "metrics-aggregator",
			"instance":       hostname + "-agg",
			"correlation_id": correlationID,
		},
		"values":     []float64{value},
		"timestamps": []int64{timestamp * 1000},
	}
}

// sendToVictoriaMetrics sends data to VictoriaMetrics
func sendToVictoriaMetrics(data map[string]interface{}, logger *log.Logger) error {

	victoriaMetrics := os.Getenv("VICTORIA_METRICS_URL")
	if victoriaMetrics == "" {
		logger.Fatal("VICTORIA_METRICS_URL environment variable is not set")
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not marshal JSON: %v", err)
	}

	req, err := http.NewRequest("POST", victoriaMetrics, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("could not create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Log the JSON for debugging
	logger.Printf("Sending JSON to VictoriaMetrics: %s", string(jsonData))
	logger.Printf("Response Status: %d", resp.StatusCode)
	logger.Printf("Response Body: %s", string(body))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected response from VictoriaMetrics: %s", resp.Status)
	}

	logger.Printf("Successfully sent metrics to VictoriaMetrics. Status: %s", resp.Status)
	return nil
}

func main() {
	logger := initLogger()
	defer func() {
		if f, ok := logger.Writer().(*os.File); ok {
			f.Close()
		}
	}()

	logger.Println("AGGREGATOR MAIN STARTED")

	err := StartAggregator(logger)
	if err != nil {
		logger.Fatal("Failed to start aggregator:", err)
	}
}
