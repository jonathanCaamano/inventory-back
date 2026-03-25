package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ── mock ─────────────────────────────────────────────────────────────────────

type mockStatsQuerier struct {
	fetchFn func() (InventoryStats, error)
}

func (m *mockStatsQuerier) Fetch() (InventoryStats, error) {
	return m.fetchFn()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newStatsRouter(q statsQuerier) *gin.Engine {
	r := gin.New()
	h := &StatsHandler{querier: q}
	r.GET("/stats", h.GetStats)
	return r
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestStatsHandler_GetStats_Success(t *testing.T) {
	q := &mockStatsQuerier{
		fetchFn: func() (InventoryStats, error) {
			return InventoryStats{
				TotalProducts:   10,
				ActiveProducts:  8,
				TotalCategories: 3,
				TotalUsers:      5,
			}, nil
		},
	}
	r := newStatsRouter(q)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/stats", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStatsHandler_GetStats_QueryError(t *testing.T) {
	q := &mockStatsQuerier{
		fetchFn: func() (InventoryStats, error) {
			return InventoryStats{}, errors.New("db connection failed")
		},
	}
	r := newStatsRouter(q)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/stats", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}
