package main

import (
	"fmt"
	"log"

	"gomon/alerting/internal/config"
	"gomon/alerting/internal/database"
	"gomon/alerting/internal/handlers"
	"gomon/alerting/internal/k8s"
	"gomon/alerting/internal/metrics"
	"gomon/alerting/internal/repository"
	"gomon/alerting/internal/server"
	"gomon/alerting/internal/slack"
)

func main() {

	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Printf("Warning: Could not initialize K8s client: %v", err)
		log.Println("Continuing without K8s monitoring...")
	} else {
		log.Println("Successfully connected to Kubernetes API")
		if k8sClient != nil {
			log.Println("Getting PODs restart statistics for namespaces: monitoring, kube-system, ingress-nginx")
			k8s.ListPods(k8sClient, "monitoring")
			k8s.ListPods(k8sClient, "kube-system")
			k8s.ListPods(k8sClient, "ingress-nginx")
		}
	}

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

	// Initialize
	metrics := metrics.NewMetrics()

	// Initialize slack connection
	var slackClient *slack.Client
	if cfg.Slack.Enabled {
		slackClient, err = slack.NewSlackClient(cfg.Slack, metrics)
		if err != nil {
			log.Printf("⚠️  Could not initialize Slack: %v", err)
			log.Println("Continuing without Slack notifications...")
		} else {
			log.Println("✅ Slack client initialized")
			// Send test message
			if err := slackClient.SendMessage("🚀 Alerting service started"); err != nil {
				log.Printf("⚠️  Failed to send test message: %v", err)
			}
		}
	} else {
		log.Println("ℹ️  Slack notifications disabled in config")
	}

	// Initialize repositories
	alertRepo := repository.NewPostgresAlertRepository(db)
	healthChecker := repository.NewPostgresHealthChecker(db)

	log.Println("Init watchers...")
	k8s.StartWatching(k8sClient, alertRepo, slackClient)

	// Initialize handlers
	alertHandler := handlers.NewAlertHandler(alertRepo, slackClient, metrics)
	healthHandler := handlers.NewHealthHandler(healthChecker)

	// Initialize and start server
	srv := server.New(alertHandler, healthHandler, cfg.Http.Port)

	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
