package server

import (
	"fmt"
	"strconv"

	"gomon/alerting/internal/handlers"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"log"
	"sync"
)

type Server struct {
	router        *gin.Engine
	alertHandler  *handlers.AlertHandler
	healthHandler *handlers.HealthHandler
	port          int
}

var registerMetricsOnce sync.Once

func New(alertHandler *handlers.AlertHandler, healthHandler *handlers.HealthHandler, port int) *Server {
	registerMetricsOnce.Do(func() {
		// This code will only run ONCE, even if initMetrics() is called multiple times
		prometheus.MustRegister(collectors.NewGoCollector())
		prometheus.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
		log.Println("Prometheus metrics registered successfully")
	})

	return &Server{
		router:        gin.Default(),
		alertHandler:  alertHandler,
		healthHandler: healthHandler,
		port:          port,
	}
}

func (s *Server) SetupRoutes() {
	// Health check
	s.router.GET("/health/database", s.healthHandler.CheckDatabase)

	// Prometheus metrics
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

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
