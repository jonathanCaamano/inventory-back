package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func openSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	return db
}

func TestHealthHandler_Live(t *testing.T) {
	db := openSQLite(t)
	h := NewHealthHandler(db, nil)

	r := gin.New()
	r.GET("/livez", h.Live)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/livez", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body["status"] != "alive" {
		t.Errorf("expected status=alive, got %q", body["status"])
	}
}

func TestHealthHandler_Health_DBUp_NoMinio(t *testing.T) {
	db := openSQLite(t)
	h := NewHealthHandler(db, nil)

	r := gin.New()
	r.GET("/health", h.Health)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("expected status ok, got %q", body.Status)
	}
	if body.Components["database"].Status != "up" {
		t.Errorf("expected database up, got %q", body.Components["database"].Status)
	}
	if body.Components["storage"].Status != "disabled" {
		t.Errorf("expected storage disabled, got %q", body.Components["storage"].Status)
	}
}

func TestHealthHandler_Health_MinioUp(t *testing.T) {
	db := openSQLite(t)
	h := NewHealthHandler(db, func() bool { return true })

	r := gin.New()
	r.GET("/health", h.Health)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body.Components["storage"].Status != "up" {
		t.Errorf("expected storage up, got %q", body.Components["storage"].Status)
	}
}

func TestHealthHandler_Health_MinioDown_StillOK(t *testing.T) {
	db := openSQLite(t)
	h := NewHealthHandler(db, func() bool { return false })

	r := gin.New()
	r.GET("/health", h.Health)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	// MinIO down is graceful degradation — DB still up = 200
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (graceful degradation), got %d", w.Code)
	}

	var body healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body.Components["storage"].Status != "down" {
		t.Errorf("expected storage down, got %q", body.Components["storage"].Status)
	}
}
