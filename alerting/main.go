package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"

	"gopkg.in/yaml.v3"

	"strconv"

	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
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

// HTTP Handler
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
	dbConfig, err := NewDatabaseConnectionPool(config)
	if err != nil {
		fmt.Printf("Failed to connect to database: %v", err)
		os.Exit(1)
	}

	// Checker
	checker := &PostgreSQLHealthChecker{db: dbConfig}

	// init Gin engine
	router := gin.Default()
	router.GET("/health/database", healthHandler(checker))

	// Start HTTP server with port from yaml
	fmt.Printf("Loaded config: Port=%d, DB=%s:%d\n", config.Http.Port, config.Db.Host, config.Db.Port)
	router.Run(":" + strconv.Itoa(config.Http.Port))
}
