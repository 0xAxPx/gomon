package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {

	// Counters
	alertsCreated      *prometheus.CounterVec // Has labels: severity, source
	slackNotifications *prometheus.CounterVec // Has labels: status (success/failure)

	// Gauges
	activeAlerts        prometheus.Gauge
	circuitBreakerState prometheus.Gauge

	// Histograms
	alertProcessingTime prometheus.Histogram
}

func NewMetrics() *Metrics {

	m := &Metrics{
		alertsCreated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "alerting_alerts_created_total",
				Help: "Total number of alerts created",
			},
			[]string{"severity", "source"},
		),
		slackNotifications: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "slack_notifications_sent_total",
				Help: "Total number of notification sent to Slack",
			},
			[]string{"status"},
		),
		activeAlerts: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "alerting_active_alerts",
				Help: "Current number of active alerts",
			},
		),

		circuitBreakerState: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alerting_circuit_breaker_state",
			Help: "Circuit breaker state (0=CLOSED, 1=OPEN, 2=HALF_OPEN)",
		},
		),

		alertProcessingTime: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name: "alerting_alert_processing_duration_seconds",
				Help: "Time taken to process an alert in seconds",
			},
		),
	}

	prometheus.MustRegister(m.alertsCreated)

	prometheus.MustRegister(m.slackNotifications)

	prometheus.MustRegister(m.activeAlerts)

	prometheus.MustRegister(m.circuitBreakerState)

	prometheus.MustRegister(m.alertProcessingTime)

	return m
}

func (m *Metrics) IncAlertsCreated(severity, source string) {
	m.alertsCreated.WithLabelValues(severity, source).Inc()
}

func (m *Metrics) IncSlackNotifications(status string) {
	m.slackNotifications.WithLabelValues(status).Inc()
}

func (m *Metrics) SetActiveAlerts(count float64) {
	m.activeAlerts.Set(count)
}

func (m *Metrics) SetCircuitBreakerState(count float64) {
	m.circuitBreakerState.Set(count)
}

func (m *Metrics) SetAlertProcessingTime(seconds float64) {
	m.alertProcessingTime.Observe(seconds)
}
