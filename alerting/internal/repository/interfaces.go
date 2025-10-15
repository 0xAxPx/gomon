package repository

import (
	"gomon/alerting/internal/models"

	"github.com/google/uuid"
)

// AlertRepository defines operations for alert persistence
type AlertRepository interface {
	Create(request models.CreateAlertRequest) (models.CreateAlertResponse, error)
	List() (models.AlertListResponse, error)
	GetByID(id uuid.UUID) (*models.Alert, error)
	GetByStatusAndSeverity(status, severity string) (models.AlertListResponse, error)
	Acknowledge(id uuid.UUID) (*models.Alert, error)
	Resolve(id uuid.UUID) (*models.Alert, error)
	Assign(id uuid.UUID, assignedTo string) (*models.Alert, error)
	Delete(id uuid.UUID) (*models.Alert, error)
	FindActiveAlertByPod(namespace, podName string) (*models.Alert, error)
}

// HealthChecker defines health check operations
type HealthChecker interface {
	CheckHealth() error
}
