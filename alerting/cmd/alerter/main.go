package main

import (
	"fmt"
	"log"

	"gomon/alerting/internal/config"
	"gomon/alerting/internal/database"
	"gomon/alerting/internal/handlers"
	"gomon/alerting/internal/repository"
	"gomon/alerting/internal/server"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database connection
	db, err := database.NewConnection(cfg.Db)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	fmt.Printf("Connected to database: %s:%d/%s\n",
		cfg.Db.Host, cfg.Db.Port, cfg.Db.Database)

	// Initialize repositories
	alertRepo := repository.NewPostgresAlertRepository(db)
	healthChecker := repository.NewPostgresHealthChecker(db)

	// Initialize handlers
	alertHandler := handlers.NewAlertHandler(alertRepo)
	healthHandler := handlers.NewHealthHandler(healthChecker)

	// Initialize and start server
	srv := server.New(alertHandler, healthHandler, cfg.Http.Port)

	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
