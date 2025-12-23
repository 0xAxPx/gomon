package server

import (
	"fmt"
	"strconv"

	"gomon/alerting/internal/handlers"

	"github.com/gin-gonic/gin"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	router         *gin.Engine
	alertHandler   *handlers.AlertHandler
	webhookHandler *handlers.WebhookHandler
	healthHandler  *handlers.HealthHandler
	port           int
}

func New(alertHandler *handlers.AlertHandler, healthHandler *handlers.HealthHandler, port int) *Server {

	webhookHandler := handlers.NewWebhookHandler(alertHandler)

	return &Server{
		router:         gin.Default(),
		alertHandler:   alertHandler,
		webhookHandler: webhookHandler,
		healthHandler:  healthHandler,
		port:           port,
	}
}

func (s *Server) SetupRoutes() {
	// Health check
	s.router.GET("/health/database", s.healthHandler.CheckDatabase)

	// Prometheus metrics
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	s.router.POST("/webhook", s.webhookHandler.HandleVMWebhook)

	// Alert routes
	api := s.router.Group("/api/v1/alerts")
	{
		api.POST("", s.alertHandler.Create)
		api.GET("", s.alertHandler.List)
		api.GET("/:id", s.alertHandler.GetByID)
		api.PUT("/:id/acknowledge", s.alertHandler.Acknowledge)
		api.PUT("/:id/resolve", s.alertHandler.Resolve)
		api.PUT("/:id/assign", s.alertHandler.Assign)
		api.DELETE("/:id", s.alertHandler.Delete)
	}
}

func (s *Server) Start() error {
	s.SetupRoutes()
	fmt.Printf("Starting server on port %d...\n", s.port)
	return s.router.Run(":" + strconv.Itoa(s.port))
}
