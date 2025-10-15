package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gomon/alerting/internal/models"

	"github.com/google/uuid"
)

type PostgresAlertRepository struct {
	db *sql.DB
}

func NewPostgresAlertRepository(db *sql.DB) *PostgresAlertRepository {
	return &PostgresAlertRepository{db: db}
}

func (r *PostgresAlertRepository) Create(request models.CreateAlertRequest) (models.CreateAlertResponse, error) {
	alertID := uuid.New()
	alertGroupID := uuid.New()

	labelsJSON, err := json.Marshal(request.Labels)
	if err != nil {
		return models.CreateAlertResponse{}, fmt.Errorf("failed to marshal labels: %w", err)
	}

	query := `INSERT INTO alerts_active (
		id, alert_group_id, source, severity, status, title, description, 
		namespace, labels, jaeger_trace_id
	) VALUES ($1, $2, $3, $4, 'firing', $5, $6, $7, $8, $9)
	RETURNING created_at`

	var createdAt time.Time
	err = r.db.QueryRow(
		query,
		alertID,
		alertGroupID,
		request.Source,
		request.Severity,
		request.Title,
		request.Description,
		request.Namespace,
		labelsJSON,
		request.TraceID,
	).Scan(&createdAt)

	if err != nil {
		return models.CreateAlertResponse{}, fmt.Errorf("failed to insert alert: %w", err)
	}

	return models.CreateAlertResponse{
		ID:           alertID.String(),
		AlertGroupID: alertGroupID.String(),
		Status:       "firing",
		CreatedAt:    createdAt.Format(time.RFC3339),
	}, nil
}

func (r *PostgresAlertRepository) List() (models.AlertListResponse, error) {
	query := `SELECT * FROM alerts_active ORDER BY created_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return models.AlertListResponse{}, fmt.Errorf("failed to query alerts: %w", err)
	}
	defer rows.Close()

	alerts, err := scanAlerts(rows)
	if err != nil {
		return models.AlertListResponse{}, err
	}

	return models.AlertListResponse{
		Alerts: alerts,
		Total:  len(alerts),
	}, nil
}

func (r *PostgresAlertRepository) GetByID(id uuid.UUID) (*models.Alert, error) {
	query := `SELECT * FROM alerts_active WHERE id=$1`

	row := r.db.QueryRow(query, id)
	alert, err := scanAlert(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("alert not found %w", id)
		}
		return nil, fmt.Errorf("failed to get alert: %w", err)
	}

	return alert, nil
}

func (r *PostgresAlertRepository) FindActiveAlertByPod(namespace string, podName string) (*models.Alert, error) {
	log.Printf("Searching for alert for %s pod and %s namespace", podName, namespace)
	query := `SELECT * 
FROM alerts_active
WHERE source = 'kubernetes'
  AND namespace = $1
  AND labels->>'pod_name' = $2
  AND status = 'firing'
ORDER BY created_at DESC
LIMIT 1 `

	row := r.db.QueryRow(query, namespace, podName)
	alert, err := scanAlert(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get alert: %w", err)
	}

	return alert, nil
}

func (r *PostgresAlertRepository) GetByStatusAndSeverity(status, severity string) (models.AlertListResponse, error) {
	log.Printf("Get status for alert with %s status and %s severity", status, severity)
	query := `SELECT * FROM alerts_active WHERE status=$1 AND severity=$2`

	rows, err := r.db.Query(query, status, severity)
	if err != nil {
		return models.AlertListResponse{}, fmt.Errorf("failed to query alerts: %w", err)
	}
	defer rows.Close()

	alerts, err := scanAlerts(rows)
	if err != nil {
		return models.AlertListResponse{}, err
	}

	return models.AlertListResponse{
		Alerts: alerts,
		Total:  len(alerts),
	}, nil
}

func (r *PostgresAlertRepository) Acknowledge(id uuid.UUID) (*models.Alert, error) {
	log.Printf("Acknowledge alert with %w", id)

	query := `
		UPDATE alerts_active 
		SET status = 'acknowledged', 
			acknowledged_at = $2, 
			acknowledged_by = 'system', 
			updated_at = NOW()
		WHERE id = $1
		RETURNING *`

	row := r.db.QueryRow(query, id, time.Now().UTC())
	return scanAlert(row)
}

func (r *PostgresAlertRepository) Resolve(id uuid.UUID) (*models.Alert, error) {
	log.Printf("Resolve alert with %s", id)
	query := `
		UPDATE alerts_active 
		SET status = 'resolved', 
			resolved_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
		RETURNING *`

	row := r.db.QueryRow(query, id)
	return scanAlert(row)
}

func (r *PostgresAlertRepository) Assign(id uuid.UUID, assignedTo string) (*models.Alert, error) {
	log.Printf("Assign alert with %w and assignt to %s", id, assignedTo)

	query := `
		UPDATE alerts_active 
		SET assigned_to = $2,
			updated_at = NOW()
		WHERE id = $1
		RETURNING *`

	row := r.db.QueryRow(query, id, assignedTo)
	return scanAlert(row)
}

func (r *PostgresAlertRepository) Delete(id uuid.UUID) (*models.Alert, error) {
	log.Printf("Delete alert with %w", id)

	query := `DELETE FROM alerts_active WHERE id = $1 RETURNING *`

	row := r.db.QueryRow(query, id)
	return scanAlert(row)
}

// Helper functions to reduce duplication
func scanAlert(row interface {
	Scan(dest ...interface{}) error
}) (*models.Alert, error) {
	var alert models.Alert
	var labelsJSON, annotationsJSON []byte

	err := row.Scan(
		&alert.ID,
		&alert.AlertGroupID,
		&alert.Source,
		&alert.Severity,
		&alert.Status,
		&alert.Title,
		&alert.Description,
		&alert.Namespace,
		&labelsJSON,
		&annotationsJSON,
		&alert.IncidentID,
		&alert.JaegerTraceID,
		&alert.CreatedAt,
		&alert.AcknowledgedAt,
		&alert.AcknowledgedBy,
		&alert.UpdatedAt,
		&alert.ResolvedAt,
		&alert.AssignedTo,
	)

	if err == sql.ErrNoRows {
		return nil, nil // ← Not found is OK
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan alert: %w", err)
	}

	return &alert, nil
}

func scanAlerts(rows *sql.Rows) ([]models.Alert, error) {
	var alerts []models.Alert

	for rows.Next() {
		var alert models.Alert
		var labelsJSON, annotationsJSON []byte

		err := rows.Scan(
			&alert.ID,
			&alert.AlertGroupID,
			&alert.Source,
			&alert.Severity,
			&alert.Status,
			&alert.Title,
			&alert.Description,
			&alert.Namespace,
			&labelsJSON,
			&annotationsJSON,
			&alert.IncidentID,
			&alert.JaegerTraceID,
			&alert.CreatedAt,
			&alert.AcknowledgedAt,
			&alert.AcknowledgedBy,
			&alert.UpdatedAt,
			&alert.ResolvedAt,
			&alert.AssignedTo,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(labelsJSON, &alert.Labels); err != nil {
			alert.Labels = make(map[string]interface{})
		}

		alerts = append(alerts, alert)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return alerts, nil
}

// PostgresHealthChecker implements health checking
type PostgresHealthChecker struct {
	db *sql.DB
}

func NewPostgresHealthChecker(db *sql.DB) *PostgresHealthChecker {
	return &PostgresHealthChecker{db: db}
}

func (h *PostgresHealthChecker) CheckHealth() error {
	return h.db.Ping()
}
