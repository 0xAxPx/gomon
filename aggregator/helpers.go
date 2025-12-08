package main

import (
	"strconv"
)

func CreateMetricData(metricName string, value float64, timestampStr string, correlationID string, hostname string) map[string]interface{} {
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
