package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLogger_LogsRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r := gin.New()
	r.GET("/test", RequestLogger(logger), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequestLogger_LogsErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r := gin.New()
	r.GET("/fail", RequestLogger(logger), func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "oops"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/fail", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
