package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"gomon/alerting/internal/models"
	"gomon/alerting/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// VictoriaMetrics webhook payload
type VMWebhookPayload struct {
	Version  string    `json:"version"`
	GroupKey string    `json:"groupKey"`
	Status   string    `json:"status"`
	Alerts   []VMAlert `json:"alerts"`
}

type VMAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

type WebhookHandler struct {
	alertHandler *AlertHandler
}

func NewWebhookHandler(alertHandler *AlertHandler) *WebhookHandler {
	return &WebhookHandler{
		alertHandler: alertHandler,
	}
}

// HandleVMWebhook processes VictoriaMetrics webhooks
func (h *WebhookHandler) HandleVMWebhook(c *gin.Context) {
	var payload VMWebhookPayload

	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Printf("Failed to parse webhook: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	log.Printf("📨 Webhook: %d alerts, status: %s", len(payload.Alerts), payload.Status)

	successCount := 0
	for _, vmAlert := range payload.Alerts {
		if err := h.processAlert(c.Request.Context(), vmAlert); err != nil {
			log.Printf("❌ Failed: %s: %v", vmAlert.Labels["alertname"], err)
		} else {
			successCount++
		}
	}

	log.Printf("✅ Processed %d/%d alerts", successCount, len(payload.Alerts))
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"processed": successCount,
		"total":     len(payload.Alerts),
	})
}

func (h *WebhookHandler) processAlert(ctx context.Context, vmAlert VMAlert) error {
	alertName := vmAlert.Labels["alertname"]
	severity := vmAlert.Labels["severity"]
	if severity == "" {
		severity = "P3" // Default
	}

	description := vmAlert.Annotations["description"]

	log.Printf("🔄 Processing: %s [%s] - %s", alertName, severity, vmAlert.Status)

	if vmAlert.Status == "firing" {
		// Convert labels to map[string]interface{}
		labels := make(map[string]interface{})
		for k, v := range vmAlert.Labels {
			labels[k] = v
		}
		labels["fingerprint"] = vmAlert.Fingerprint
		labels["generator_url"] = vmAlert.GeneratorURL

		// Convert annotations
		annotations := make(map[string]interface{})
		for k, v := range vmAlert.Annotations {
			annotations[k] = v
		}

		// Generate consistent AlertGroupID from fingerprint
		alertGroupID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(vmAlert.Fingerprint))

		alert := &models.Alert{
			ID:           uuid.New(),
			AlertGroupID: alertGroupID,
			Source:       "victoriametrics",
			Severity:     severity,
			Status:       "firing",
			Title:        alertName,
			Description:  &description,
			Labels:       labels,
			Annotations:  annotations,
			CreatedAt:    vmAlert.StartsAt,
			UpdatedAt:    time.Now(),
		}

		// Extract namespace if present
		if namespace, ok := vmAlert.Labels["namespace"]; ok {
			alert.Namespace = &namespace
		}

		// Extract trace ID if present
		if traceID, ok := vmAlert.Labels["trace_id"]; ok {
			alert.JaegerTraceID = &traceID
		}

		// Create using repository
		repo := h.alertHandler.repo.(*repository.PostgresAlertRepository)
		if err := repo.CreateInternal(alert); err != nil {
			return fmt.Errorf("failed to create alert: %w", err)
		}

		// Update metrics
		h.alertHandler.mu.Lock()
		h.alertHandler.activeAlertCount++
		h.alertHandler.mu.Unlock()

		if h.alertHandler.metrics != nil {
			h.alertHandler.metrics.IncAlertsCreated(severity, "victoriametrics")
			h.alertHandler.metrics.SetActiveAlerts(float64(h.alertHandler.activeAlertCount))
		}

		// Send Slack notification
		if h.alertHandler.slackClient != nil {
			go func() {
				message := fmt.Sprintf(
					"🚨 *%s*\n*Severity:* %s\n*Description:* %s",
					alertName, severity, description,
				)
				if err := h.alertHandler.slackClient.SendMessage(message); err != nil {
					log.Printf("⚠️ Slack failed: %v", err)
				}
			}()
		}

		log.Printf("✅ Created: %s (ID: %s)", alertName, alert.ID)

	} else if vmAlert.Status == "resolved" {
		log.Printf("🔵 Resolving: %s", alertName)

		repo := h.alertHandler.repo.(*repository.PostgresAlertRepository)
		if err := repo.ResolveByFingerprint(vmAlert.Fingerprint); err != nil {
			return fmt.Errorf("failed to resolve: %w", err)
		}

		// Update metrics
		h.alertHandler.mu.Lock()
		if h.alertHandler.activeAlertCount > 0 {
			h.alertHandler.activeAlertCount--
		}
		h.alertHandler.mu.Unlock()

		if h.alertHandler.metrics != nil {
			h.alertHandler.metrics.SetActiveAlerts(float64(h.alertHandler.activeAlertCount))
		}
	}
	return nil
}
