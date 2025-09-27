package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"gopkg.in/yaml.v3"
)

// Config
type Config struct {
	Http HttpConfig `yaml:"http"` // Maps to "http:" section
	Db   DbConfig   `yaml:"db"`   // Maps to "db:" section
}

type HttpConfig struct {
	Port int `yaml:"port"`
}

type DbConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// Funct to read config and construct Config object
func readYml() (Config, error) {

	// Read yaml
	yamlFile := "configs/prod.yaml"
	byteYaml, err := os.ReadFile(yamlFile)
	if err != nil {
		fmt.Printf("ERROR: could not read %s", yamlFile)
		return Config{}, err
	}

	var config Config

	// Unmarshall: yml -> Config
	err = yaml.Unmarshal(byteYaml, &config)
	return config, err

}

// Health Checks
type DatabaseHealthChecker interface {
	CheckHealth() error
}

// Postgress
type PostgreSQLHealthChecker struct {
	db *sql.DB
}

func (p *PostgreSQLHealthChecker) CheckHealth() error {
	return p.db.Ping()
}

// Connection pool creation
func NewDatabaseConnectionPool(config Config) (*sql.DB, error) {

	// Data source name
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Db.Host, config.Db.Port, config.Db.User, config.Db.Password, config.Db.Database)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	return db, err
}

// Alerts
type CreateAlertRequest struct {
	Source      string
	Severity    string
	Title       string
	Description string
	Namespace   string
	Labels      map[string]string
	TraceID     string
}

type CreateAlertResponse struct {
	ID           string
	AlertGroupID string
	Status       string
	CreatedAt    string
}

type UpdateAlertResponse struct {
	ID             uuid.UUID `json:"id"`
	AlertGroupID   uuid.UUID `json:"alert_group_id"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	AcknowledgedAt time.Time `json:"acknowledged_at"`
	AcknowledgedBy string    `json:"acknowledged_by"`
}

type AlertCreator interface {
	CreateAlert(request CreateAlertRequest) (CreateAlertResponse, error)
	ListAlerts() (AlertListResponse, error)
	GetAlertByID(id uuid.UUID) (AlertListResponse, error)
	AcknowledgeAlert(id uuid.UUID) (*Alert, error)
}

type DatabaseAlertCreator struct {
	db *sql.DB
}

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

type AlertListResponse struct {
	Alerts []Alert `json:"alerts"`
	Total  int     `json:"total"`
}

func (d *DatabaseAlertCreator) CreateAlert(request CreateAlertRequest) (CreateAlertResponse, error) {

	alertID := uuid.New()
	alertGroupID := uuid.New()

	labelsJSON, err := json.Marshal(request.Labels)
	if err != nil {
		return CreateAlertResponse{}, fmt.Errorf("failed to marshall labels: %w", err)
	}

	query := `INSERT INTO alerts_active (
		id, alert_group_id, source, severity, status, title, description, 
		namespace, labels, jaeger_trace_id
	) VALUES ($1, $2, $3, $4, 'firing', $5, $6, $7, $8, $9)
	RETURNING created_at`

	var createdAt time.Time
	err = d.db.QueryRow(
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
		return CreateAlertResponse{}, fmt.Errorf("failed to insert alert: %w", err)
	}

	return CreateAlertResponse{
		ID:           alertID.String(),
		AlertGroupID: alertGroupID.String(),
		Status:       "firing",
		CreatedAt:    createdAt.Format(time.RFC3339),
	}, nil

}

// HTTP Handler for alert
func createAlertHandler(alertCreator AlertCreator) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Parse request
		var request CreateAlertRequest
		if err := ctx.ShouldBindJSON(&request); err != nil {
			ctx.JSON(400, gin.H{"error": "Invalid JSON"})
			return
		}

		// Save alert
		response, err := alertCreator.CreateAlert(request)
		if err != nil {
			if isDatabaseConstraintError(err) {
				ctx.JSON(400, gin.H{"error": "Invalid alert data", "details": err.Error()})
				return
			}
			ctx.JSON(500, gin.H{"error": "Database operation failed"})
			return
		}

		// Return 201 HTTP
		ctx.JSON(201, response)

	}
}

func isDatabaseConstraintError(err error) bool {
	errorStr := err.Error()
	return strings.Contains(errorStr, "constraint") ||
		strings.Contains(errorStr, "duplicate") ||
		strings.Contains(errorStr, "violates")
}

func (d *DatabaseAlertCreator) ListAlerts() (AlertListResponse, error) {
	query := `SELECT * FROM alerts_active ORDER BY created_at DESC`

	rows, err := d.db.Query(query)
	if err != nil {
		return AlertListResponse{}, fmt.Errorf("failed to query alerts: %w", err)
	}

	defer rows.Close()

	var alerts []Alert

	for rows.Next() {
		var alert Alert
		var labelsJSON []byte
		var annotationsJSON []byte

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
			return AlertListResponse{}, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(labelsJSON, &alert.Labels); err != nil {
			alert.Labels = make(map[string]interface{})
		}

		alerts = append(alerts, alert)
	}

	if err = rows.Err(); err != nil {
		return AlertListResponse{}, fmt.Errorf("error during row iteration: %w", err)
	}

	return AlertListResponse{
		Alerts: alerts,
		Total:  len(alerts),
	}, nil

}

func createAlertListHandler(alertCreator AlertCreator) gin.HandlerFunc { // Add parameter
	return func(ctx *gin.Context) {
		response, err := alertCreator.ListAlerts() // Call method on interface
		if err != nil {
			ctx.JSON(500, gin.H{"error": "Can't return data from db", "details": err.Error()})
			return
		}

		// Don't forget to return the response!
		ctx.JSON(200, response)
	}
}

func (d *DatabaseAlertCreator) GetAlertByID(id uuid.UUID) (AlertListResponse, error) {
	query := `SELECT * FROM alerts_active where id=$1`

	rows, err := d.db.Query(query, id.String())
	if err != nil {
		return AlertListResponse{}, fmt.Errorf("failed to query alerts: %w", err)
	}

	defer rows.Close()

	var alerts []Alert

	for rows.Next() {
		var alert Alert
		var labelsJSON []byte
		var annotationsJSON []byte

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
			return AlertListResponse{}, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(labelsJSON, &alert.Labels); err != nil {
			alert.Labels = make(map[string]interface{})
		}

		alerts = append(alerts, alert)
	}

	if err = rows.Err(); err != nil {
		return AlertListResponse{}, fmt.Errorf("error during row iteration: %w", err)
	}

	return AlertListResponse{
		Alerts: alerts,
		Total:  len(alerts),
	}, nil

}

func createAlertByIdHandler(alertCreator AlertCreator) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Step 1: Extract ID from URL path
		idStr := ctx.Param("id")

		// Step 2: Parse string to UUID
		alertID, err := uuid.Parse(idStr)
		if err != nil {
			ctx.JSON(400, gin.H{"error": "Invalid alert ID format"})
			return
		}

		// Step 3: Call GetAlertByID method
		response, err := alertCreator.GetAlertByID(alertID)
		if err != nil {
			ctx.JSON(500, gin.H{"error": "Failed to get alert", "details": err.Error()})
			return
		}

		// Step 4: Handle not found case
		if len(response.Alerts) == 0 {
			ctx.JSON(404, gin.H{"error": "Alert not found"})
			return
		}

		// Step 5: Return single alert (not wrapped in array)
		ctx.JSON(200, response.Alerts[0])
	}
}

// HTTP Handler for database
func healthHandler(checker DatabaseHealthChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := checker.CheckHealth()
		if err != nil {
			c.JSON(500, gin.H{"status": "unhealthy", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "healthy", "database": "connected"})
	}
}

func (d *DatabaseAlertCreator) AcknowledgeAlert(id uuid.UUID) (*Alert, error) {
	acknowledgeAt := time.Now().UTC()
	acknowledgeBy := "test_user"
	status := "acknowledged"

	query := `
        UPDATE alerts_active 
        SET status = $2, 
            acknowledged_at = $3, 
            acknowledged_by = $4, 
            updated_at = NOW()
        WHERE id = $1
        RETURNING *`

	row := d.db.QueryRow(query, id, status, acknowledgeAt, acknowledgeBy)

	var alert Alert
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

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("alert not found")
		}
		return nil, fmt.Errorf("failed to acknowledge alert: %w", err)
	}

	json.Unmarshal(labelsJSON, &alert.Labels)

	return &alert, nil
}

func acknowledgeAlertHandler(alertCreator AlertCreator) gin.HandlerFunc {
	return func(ctx *gin.Context) {

		// Step 1: Extract ID from URL path
		idStr := ctx.Param("id")

		// Step 2: Parse string to UUID
		alertID, err := uuid.Parse(idStr)
		if err != nil {
			ctx.JSON(400, gin.H{"error": "Invalid alert ID format"})
			return
		}

		// Step 3: Call GetAlertByID method
		response, err := alertCreator.AcknowledgeAlert(alertID)
		if err != nil {
			ctx.JSON(500, gin.H{"error": "Failed to get alert", "details": err.Error()})
			return
		}

		ctx.JSON(200, response)

	}
}

// main
func main() {
	// Construct config
	config, err := readYml()
	if err != nil {
		fmt.Printf("Yaml config was not read!")
		os.Exit(1)
	}

	// Create connetion pool
	dbPool, err := NewDatabaseConnectionPool(config)
	if err != nil {
		fmt.Printf("Failed to connect to database: %v", err)
		os.Exit(1)
	}

	// Checker
	checker := &PostgreSQLHealthChecker{db: dbPool}
	alertCreator := &DatabaseAlertCreator{db: dbPool}

	// init Gin engine and add routing
	router := gin.Default()
	var api = "/api/v1/alerts"
	router.GET("/health/database", healthHandler(checker))
	router.POST(api, createAlertHandler(alertCreator))
	router.GET(api, createAlertListHandler(alertCreator))
	router.GET(api+"/:id", createAlertByIdHandler(alertCreator))
	router.PUT(api+"/:id/acknowledge", acknowledgeAlertHandler(alertCreator))

	// Start HTTP server with port from yaml
	fmt.Printf("Loaded config: Port=%d, DB=%s:%d\n", config.Http.Port, config.Db.Host, config.Db.Port)
	router.Run(":" + strconv.Itoa(config.Http.Port))
}
