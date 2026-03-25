package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	r := gin.New()
	r.GET("/", RequestID(), func(c *gin.Context) {
		id := c.GetString(RequestIDKey)
		if id == "" {
			c.Status(http.StatusInternalServerError)
			return
		}
		// ID should appear in response header
		if c.Writer.Header().Get(RequestIDHeader) == "" {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Header().Get(RequestIDHeader) == "" {
		t.Error("expected X-Request-ID header in response")
	}
}

func TestRequestID_ReusesExistingHeader(t *testing.T) {
	customID := "my-custom-request-id-123"

	r := gin.New()
	r.GET("/", RequestID(), func(c *gin.Context) {
		id := c.GetString(RequestIDKey)
		if id != customID {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, customID)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Header().Get(RequestIDHeader) != customID {
		t.Errorf("expected response header to echo %q, got %q", customID, w.Header().Get(RequestIDHeader))
	}
}
