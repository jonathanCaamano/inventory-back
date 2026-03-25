package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db      *gorm.DB
	minioOK func() bool // optional check, nil means MinIO not configured
}

func NewHealthHandler(db *gorm.DB, minioCheck func() bool) *HealthHandler {
	return &HealthHandler{db: db, minioOK: minioCheck}
}

type componentStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type healthResponse struct {
	Status     string                     `json:"status"`
	Components map[string]componentStatus `json:"components"`
	Timestamp  time.Time                  `json:"timestamp"`
}

// Health returns detailed dependency health. Used for K8s readiness probe.
func (h *HealthHandler) Health(c *gin.Context) {
	components := make(map[string]componentStatus)
	allOK := true

	// Database check
	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.PingContext(context.Background()) != nil {
		components["database"] = componentStatus{Status: "down", Message: "ping failed"}
		allOK = false
	} else {
		components["database"] = componentStatus{Status: "up"}
	}

	// MinIO check (optional)
	if h.minioOK != nil {
		if h.minioOK() {
			components["storage"] = componentStatus{Status: "up"}
		} else {
			components["storage"] = componentStatus{Status: "down", Message: "bucket check failed"}
			// MinIO down is a warning, not critical failure (graceful degradation)
		}
	} else {
		components["storage"] = componentStatus{Status: "disabled"}
	}

	status := "ok"
	code := http.StatusOK
	if !allOK {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, healthResponse{
		Status:     status,
		Components: components,
		Timestamp:  time.Now().UTC(),
	})
}

// Live is a simple liveness probe — always 200 if the process is running.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}
