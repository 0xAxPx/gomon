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

type AlertCreator interface {
	CreateAlert(request CreateAlertRequest) (CreateAlertResponse, error)
}

type DatabaseAlertCreator struct {
	db *sql.DB
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
	alertSaver := &DatabaseAlertCreator{db: dbPool}

	// init Gin engine and add routing
	router := gin.Default()
	router.GET("/health/database", healthHandler(checker))
	router.POST("/api/v1/alerts", createAlertHandler(alertSaver))

	// Start HTTP server with port from yaml
	fmt.Printf("Loaded config: Port=%d, DB=%s:%d\n", config.Http.Port, config.Db.Host, config.Db.Port)
	router.Run(":" + strconv.Itoa(config.Http.Port))
}
