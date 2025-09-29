package handlers

import (
	"gomon/alerting/internal/repository"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	checker repository.HealthChecker
}

func NewHealthHandler(checker repository.HealthChecker) *HealthHandler {
	return &HealthHandler{checker: checker}
}

func (h *HealthHandler) CheckDatabase(ctx *gin.Context) {
	err := h.checker.CheckHealth()
	if err != nil {
		ctx.JSON(500, gin.H{
			"status":   "unhealthy",
			"database": "disconnected",
			"error":    err.Error(),
		})
		return
	}

	ctx.JSON(200, gin.H{
		"status":   "healthy",
		"database": "connected",
	})
}
