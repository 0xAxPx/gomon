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
			aggregatorReceivedTime := time.Now().UTC()
			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				logger.Printf("Could not read message: %v", err)
				continue
			}

			err = processAndSendMetrics(msg.Value, logger, aggregatorReceivedTime, tracer)
			if err != nil {
				logger.Printf("Error processing message: %v", err)
			}
		}
	}
}

func sendNetStats(metric *pb.Metric, correlationID string, rootSpan opentracing.Span, logger *log.Logger) error {
	for _, net := range metric.NetStats {
		if net != nil {

			err := sendMetricToVictoria("int_bytes_recv_mb", float32(net.BytesReceived>>20), metric.Timestamp, correlationID, rootSpan, logger)
			if err != nil {
				return fmt.Errorf("error sending Cumulative Inet Bytes Recv (MB) metric: %v", err)
			} else {
				log.Println("Successfully sent Inet Bytes Recv (MB) metrics to VictoriaMetrics")
			}

			err = sendMetricToVictoria("int_bytes_sent_mb", float32(net.BytesSent>>20), metric.Timestamp, correlationID, rootSpan, logger)
			if err != nil {
				return fmt.Errorf("error sending Cumulative Inet Bytes Sent (MB) metric: %v", err)
			} else {
				log.Println("Successfully sent Cumulative Inet Bytes Sent (MB) metrics to VictoriaMetrics")
			}
		}
	}
	return nil
}

// processAndSendMetrics processes and sends separate metrics to VictoriaMetrics
func processAndSendMetrics(protoData []byte, logger *log.Logger, aggregatorReceivedTime time.Time, tracer opentracing.Tracer) error {
	var metric pb.Metric
	err := proto.Unmarshal(protoData, &metric)
	if err != nil {
		return fmt.Errorf("could not unmarshal protobuf data: %v", err)
	}

	//kafka-consume span

	correlationID := metric.CorrelationId

	// Create NEW root span for aggregator
	aggregatorRootSpan := tracer.StartSpan("gomon-aggregator-processing")
	defer aggregatorRootSpan.Finish()
	aggregatorRootSpan.SetTag("correlation_id", correlationID)

	kafkaLatency := time.Since(aggregatorReceivedTime)
	logger.Printf("Kafka vs Aggregator publish latency: %v (CorrelationID: %s)",
		kafkaLatency, correlationID)

	kafkaSpan := opentracing.StartSpan("kafka-consume", opentracing.ChildOf(aggregatorRootSpan.Context()))
	kafkaSpan.SetTag("kafka-consume-latency", kafkaLatency)
	kafkaSpan.SetTag("success", true)
	kafkaSpan.Finish()

	err = sendMetricToVictoria("cpu_usage_percent", metric.CpuUsagePercent, metric.Timestamp, correlationID, aggregatorRootSpan, logger)
	if err != nil {
		return fmt.Errorf("error sending CPU metric: %v", err)
	} else {
		logger.Println("Successfully sent CPU metrics to VictoriaMetrics")
	}

	err = sendMetricToVictoria("mem_usage_percent", metric.MemoryUsedPercent, metric.Timestamp, correlationID, aggregatorRootSpan, logger)
	if err != nil {
		return fmt.Errorf("error sending MemUsage metric: %v", err)
	} else {
		logger.Println("Successfully sent Mem metrics to VictoriaMetrics")
	}

	err = sendMetricToVictoria("dsk_used_gb", float32(metric.MemoryUsedGb), metric.Timestamp, correlationID, aggregatorRootSpan, logger)
	if err != nil {
		return fmt.Errorf("error sending Disk Used GB metric: %v", err)
	} else {
		logger.Println("Successfully sent Disk Used (GB) metrics to VictoriaMetrics")
	}

	// Iterating Disk stats
	for _, disk := range metric.DiskStats {
		if disk != nil {
			err = sendMetricToVictoria("disk_used_percent", float32(disk.UsedPercent), metric.Timestamp, correlationID, aggregatorRootSpan, logger)
			if err != nil {
				return fmt.Errorf("error sending Disk Used Percent metric: %v", err)
			} else {
				logger.Println("Successfully sent Disk Used(%) metrics to VictoriaMetrics")
			}
		}
	}

	// Iterating Net stats
	if err := sendNetStats(&metric, correlationID, aggregatorRootSpan, logger); err != nil {
		return fmt.Errorf("error sending Net Stats: %v", err)
	}

	// for _, net := range metric.NetStats {
	// 	if net != nil {

	// 		err = sendMetricToVictoria("int_bytes_recv_mb", float32(net.BytesReceived>>20), metric.Timestamp)
	// 		if err != nil {
	// 			return fmt.Errorf("error sending Cumulative Inet Bytes Recv (MB) metric: %v", err)
	// 		} else {
	// 			log.Println("Successfully sent Inet Bytes Recv (MB) metrics to VictoriaMetrics")
	// 		}

	// 		err = sendMetricToVictoria("int_bytes_sent_mb", float32(net.BytesSent>>20), metric.Timestamp)
	// 		if err != nil {
	// 			return fmt.Errorf("error sending Cumulative Inet Bytes Sent (MB) metric: %v", err)
	// 		} else {
	// 			log.Println("Successfully sent Cumulative Inet Bytes Sent (MB) metrics to VictoriaMetrics")
	// 		}
	// 	}
	// }

	logger.Println("Successfully processed and sent metrics to VictoriaMetrics")
	return nil
}

// sendMetricToVictoria sends individual metrics to VictoriaMetrics
func sendMetricToVictoria(metricName string, value float32, timestampStr string, correlationID string, rootSpan opentracing.Span, logger *log.Logger) error {
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %v", err)
	}

	// Get Hostname
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("error getting hostname: %v", err)
	}

	timeToProcess := time.Now().UTC()

	// Prepare the payload for VictoriaMetrics
	data := map[string]interface{}{
		"metric": map[string]string{
			"__name__":       metricName,
			"job":            "metrics-aggregator",
			"instance":       hostname + "-agg",
			"correlation_id": correlationID,
		},
		"values":     []float64{float64(value)},
		"timestamps": []int64{timestamp * 1000},
	}

	// Log the JSON for debugging
	jsonData, _ := json.Marshal(data)
	logger.Printf("Sending JSON to VictoriaMetrics: %s\n", string(jsonData))

	metricSpan := opentracing.StartSpan(metricName+"-processing", opentracing.ChildOf(rootSpan.Context()))
	metricSpan.SetTag("latency_ms", time.Since(timeToProcess))
	metricSpan.Finish()

	// Send data to VictoriaMetrics
	return sendToVictoriaMetrics(data, rootSpan, logger)
}

// sendToVictoriaMetrics sends data to VictoriaMetrics
func sendToVictoriaMetrics(data map[string]interface{}, rootSpan opentracing.Span, logger *log.Logger) error {

	victoriaMetrics := os.Getenv("VICTORIA_METRICS_URL")
	if victoriaMetrics == "" {
		logger.Fatal("VICTORIA_METRICS_URL environment variable is not set")
	}

	metricName := data["metric"].(map[string]string)["__name__"]
	aggregatorVmSpan := opentracing.StartSpan(metricName+"-agg-VM", opentracing.ChildOf(rootSpan.Context()))

	startTimeToSendToVM := time.Now().UTC()

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
		aggregatorVmSpan.SetTag("error", true)
		aggregatorVmSpan.Finish()
		rootSpan.Finish()
		return fmt.Errorf("could not send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	logger.Printf("Request Body: %s", string(jsonData))
	logger.Printf("Response Status: %d", resp.StatusCode)
	logger.Printf("Response Body: %s", string(body))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		aggregatorVmSpan.SetTag("latency_ms", time.Since(startTimeToSendToVM))
		aggregatorVmSpan.Finish()
		logger.Printf("Successfully sent metrics to VictoriaMetrics. Status: %s", resp.Status)
	} else {
		aggregatorVmSpan.SetTag("error", true)
		aggregatorVmSpan.Finish()
		return fmt.Errorf("unexpected response from VictoriaMetrics: %s", resp.Status)
	}

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
