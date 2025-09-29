package models

import (
	"time"

	"github.com/google/uuid"
)

type Alert struct {
	ID             uuid.UUID              `json:"id"`
	AlertGroupID   uuid.UUID              `json:"alert_group_id"`
	Source         string                 `json:"source"`
	Severity       string                 `json:"severity"`
	Status         string                 `json:"status"`
	Title          string                 `json:"title"`
	Description    *string                `json:"description,omitempty"`
	Namespace      *string                `json:"namespace,omitempty"`
	Labels         map[string]interface{} `json:"labels"`
	Annotations    map[string]interface{} `json:"annotations"`
	IncidentID     *uuid.UUID             `json:"incident_id,omitempty"`
	JaegerTraceID  *string                `json:"jaeger_trace_id,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	AcknowledgedAt *time.Time             `json:"acknowledged_at,omitempty"`
	AcknowledgedBy *string                `json:"acknowledged_by,omitempty"`
	UpdatedAt      time.Time              `json:"updated_at"`
	ResolvedAt     *time.Time             `json:"resolved_at,omitempty"`
	AssignedTo     *string                `json:"assigned_to,omitempty"`
}

// Request/Response DTOs
type CreateAlertRequest struct {
	Source      string            `json:"source" binding:"required"`
	Severity    string            `json:"severity" binding:"required"`
	Title       string            `json:"title" binding:"required"`
	Description string            `json:"description"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels"`
	TraceID     string            `json:"trace_id"`
}

type CreateAlertResponse struct {
	ID           string `json:"id"`
	AlertGroupID string `json:"alert_group_id"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

type AssignAlertRequest struct {
	AssignedTo string `json:"assigned_to" binding:"required"`
	Note       string `json:"note,omitempty"`
}

type AlertListResponse struct {
	Alerts []Alert `json:"alerts"`
	Total  int     `json:"total"`
}
