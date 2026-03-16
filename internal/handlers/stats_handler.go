package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type statsQuerier interface {
	Fetch() (InventoryStats, error)
}

type StatsHandler struct {
	querier statsQuerier
}

func NewStatsHandler(db *gorm.DB) *StatsHandler {
	return &StatsHandler{querier: &gormStatsQuerier{db: db}}
}

type InventoryStats struct {
	TotalProducts   int64 `json:"total_products"`
	ActiveProducts  int64 `json:"active_products"`
	TotalCategories int64 `json:"total_categories"`
	TotalUsers      int64 `json:"total_users"`
}

func (h *StatsHandler) GetStats(c *gin.Context) {
	stats, err := h.querier.Fetch()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// gormStatsQuerier implements statsQuerier using a real *gorm.DB.
type gormStatsQuerier struct {
	db *gorm.DB
}

func (q *gormStatsQuerier) Fetch() (InventoryStats, error) {
	var stats InventoryStats

	type productAgg struct {
		Total  int64
		Active int64
	}
	var agg productAgg
	if err := q.db.Raw(`
		SELECT
			COUNT(*)                              AS total,
			COUNT(*) FILTER (WHERE active = true) AS active
		FROM products
		WHERE deleted_at IS NULL
	`).Scan(&agg).Error; err != nil {
		return stats, err
	}

	stats.TotalProducts = agg.Total
	stats.ActiveProducts = agg.Active

	if err := q.db.Table("categories").Where("deleted_at IS NULL").Count(&stats.TotalCategories).Error; err != nil {
		return stats, err
	}
	if err := q.db.Table("users").Where("deleted_at IS NULL").Count(&stats.TotalUsers).Error; err != nil {
		return stats, err
	}

	return stats, nil
}
