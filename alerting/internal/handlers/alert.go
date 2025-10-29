package handlers

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"gomon/alerting/internal/metrics"
	"gomon/alerting/internal/models"
	"gomon/alerting/internal/repository"
	"gomon/alerting/internal/slack"
	"gomon/alerting/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AlertHandler struct {
	repo             repository.AlertRepository
	slackClient      *slack.Client
	metrics          *metrics.Metrics
	activeAlertCount int64
	mu               sync.Mutex
}

func NewAlertHandler(repo repository.AlertRepository, slackClient *slack.Client, metrics *metrics.Metrics) *AlertHandler {
	return &AlertHandler{repo: repo, slackClient: slackClient, metrics: metrics}
}

func (h *AlertHandler) Create(ctx *gin.Context) {
	start := time.Now()
	var request models.CreateAlertRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid JSON", "details": err.Error()})
		return
	}

	// Save to database
	response, err := h.repo.Create(request)
	if err != nil {
		if isDatabaseConstraintError(err) {
			ctx.JSON(400, gin.H{"error": "Invalid alert data", "details": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": "Database operation failed", "details": err.Error()})
		return
	}

	// Increment active alerts counter
	h.mu.Lock()
	h.activeAlertCount++
	count := h.activeAlertCount
	h.mu.Unlock()
	h.metrics.SetActiveAlerts(float64(count))

	// Calculate processing time
	duration := time.Since(start).Seconds()
	h.metrics.SetAlertProcessingTime(duration)

	// Increment alerts created counter
	h.metrics.IncAlertsCreated(request.Severity, request.Source)

	// Send to Slack
	if h.slackClient != nil {
		go h.sendSlackNotification(request, response, h.metrics)
	}

	ctx.JSON(201, response)
}

func (h *AlertHandler) List(ctx *gin.Context) {
	// Check for query parameters for filtering
	status := ctx.Query("status")
	severity := ctx.Query("severity")

	if status != "" && severity != "" {
		fmt.Printf("Filtering: status=%s, severity=%s\n", status, severity)
		response, err := h.repo.GetByStatusAndSeverity(status, severity)
		if err != nil {
			ctx.JSON(500, gin.H{"error": "Failed to get filtered alerts", "details": err.Error()})
			return
		}
		ctx.JSON(200, response)
		return
	}

	// Return all alerts
	response, err := h.repo.List()
	if err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to retrieve alerts", "details": err.Error()})
		return
	}

	ctx.JSON(200, response)
}

func (h *AlertHandler) GetByID(ctx *gin.Context) {
	idStr := ctx.Param("id")
	alertID, err := uuid.Parse(idStr)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid alert ID format"})
		return
	}

	alert, err := h.repo.GetByID(alertID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			ctx.JSON(404, gin.H{"error": "Alert not found"})
			return
		}
		ctx.JSON(500, gin.H{"error": "Failed to get alert", "details": err.Error()})
		return
	}

	ctx.JSON(200, alert)
}

func (h *AlertHandler) FindActiveAlertByPod(ctx *gin.Context) {
	namespace := ctx.Param("namespace")
	podName := ctx.Param("podname")

	alert, err := h.repo.FindActiveAlertByPod(namespace, podName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			ctx.JSON(404, gin.H{"error": "Alert not found"})
			return
		}
		ctx.JSON(500, gin.H{"error": "Failed to get alert", "details": err.Error()})
		return
	}

	ctx.JSON(200, alert)

}

func (h *AlertHandler) Acknowledge(ctx *gin.Context) {
	idStr := ctx.Param("id")
	alertID, err := uuid.Parse(idStr)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid alert ID format"})
		return
	}

	alert, err := h.repo.Acknowledge(alertID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			ctx.JSON(404, gin.H{"error": "Alert not found"})
			return
		}
		ctx.JSON(500, gin.H{"error": "Failed to acknowledge alert", "details": err.Error()})
		return
	}

	ctx.JSON(200, alert)
}

func (h *AlertHandler) Resolve(ctx *gin.Context) {
	idStr := ctx.Param("id")
	alertID, err := uuid.Parse(idStr)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid alert ID format"})
		return
	}

	alert, err := h.repo.Resolve(alertID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			ctx.JSON(404, gin.H{"error": "Alert not found"})
			return
		}
		ctx.JSON(500, gin.H{"error": "Failed to resolve alert", "details": err.Error()})
		return
	}

	//Decrement active alerts
	h.mu.Lock()
	h.activeAlertCount--
	count := h.activeAlertCount
	h.mu.Unlock()
	h.metrics.SetActiveAlerts(float64(count))

	ctx.JSON(200, alert)
}

func (h *AlertHandler) Assign(ctx *gin.Context) {
	idStr := ctx.Param("id")
	alertID, err := uuid.Parse(idStr)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid alert ID format"})
		return
	}

	var request models.AssignAlertRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid JSON body", "details": err.Error()})
		return
	}

	alert, err := h.repo.Assign(alertID, request.AssignedTo)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			ctx.JSON(404, gin.H{"error": "Alert not found"})
			return
		}
		ctx.JSON(500, gin.H{"error": "Failed to assign alert", "details": err.Error()})
		return
	}

	ctx.JSON(200, alert)
}

func (h *AlertHandler) Delete(ctx *gin.Context) {
	idStr := ctx.Param("id")
	alertID, err := uuid.Parse(idStr)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid alert ID format"})
		return
	}

	alert, err := h.repo.Delete(alertID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			ctx.JSON(404, gin.H{"error": "Alert not found"})
			return
		}
		ctx.JSON(500, gin.H{"error": "Failed to delete alert", "details": err.Error()})
		return
	}

	ctx.JSON(200, alert)
}

// Helper function
func isDatabaseConstraintError(err error) bool {
	errorStr := err.Error()
	return strings.Contains(errorStr, "constraint") ||
		strings.Contains(errorStr, "duplicate") ||
		strings.Contains(errorStr, "violates")
}

func (h *AlertHandler) sendSlackNotification(request models.CreateAlertRequest, response models.CreateAlertResponse, metrics *metrics.Metrics) {
	// Check if this severity should trigger Slack notification
	if !utils.ShouldNotifySlack(request.Severity) {
		log.Printf("⏭️  Skipping Slack notification for severity: %s", request.Severity)
		return
	}

	// Get appropriate channel based on severity
	channels := h.slackClient.GetChannels()
	channel := utils.GetChannelForSeverity(request.Severity, channels)

	// Build alert message
	message := fmt.Sprintf(
		"🚨 *%s Alert Created via API*\n"+
			"*ID:* %s\n"+
			"*Title:* %s\n"+
			"*Source:* %s\n"+
			"*Namespace:* %s\n"+
			"*Description:* %s\n"+
			"*Status:* %s\n"+
			"*Created:* %s",
		request.Severity,
		response.ID,
		request.Title,
		request.Source,
		request.Namespace,
		request.Description,
		response.Status,
		response.CreatedAt,
	)

	// Send to Slack (circuit breaker is inside SendMessageToChannel)
	err := h.slackClient.SendMessageToChannel(message, channel)
	if err != nil {
		log.Printf("⚠️  Slack notification failed for alert %s: %v", response.ID, err)
		h.metrics.IncSlackNotifications("failure")
	} else {
		h.metrics.IncSlackNotifications("success")
	}
}
