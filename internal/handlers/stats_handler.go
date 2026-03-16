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
	TotalProducts   int64   `json:"total_products"`
	ActiveProducts  int64   `json:"active_products"`
	TotalCategories int64   `json:"total_categories"`
	TotalStock      int64   `json:"total_stock"`
	StockValue      float64 `json:"stock_value"`
	OutOfStock      int64   `json:"out_of_stock"`
	LowStock        int64   `json:"low_stock"` // stock <= 5
	TotalUsers      int64   `json:"total_users"`
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
		Total      int64
		Active     int64
		TotalStock int64
		StockValue float64
		OutOfStock int64
		LowStock   int64
	}
	var agg productAgg
	if err := q.db.Raw(`
		SELECT
			COUNT(*)                                        AS total,
			COUNT(*) FILTER (WHERE active = true)           AS active,
			COALESCE(SUM(stock), 0)                         AS total_stock,
			COALESCE(SUM(price * stock), 0)                 AS stock_value,
			COUNT(*) FILTER (WHERE stock = 0)               AS out_of_stock,
			COUNT(*) FILTER (WHERE stock > 0 AND stock <= 5) AS low_stock
		FROM products
		WHERE deleted_at IS NULL
	`).Scan(&agg).Error; err != nil {
		return stats, err
	}

	stats.TotalProducts = agg.Total
	stats.ActiveProducts = agg.Active
	stats.TotalStock = agg.TotalStock
	stats.StockValue = agg.StockValue
	stats.OutOfStock = agg.OutOfStock
	stats.LowStock = agg.LowStock

	if err := q.db.Table("categories").Where("deleted_at IS NULL").Count(&stats.TotalCategories).Error; err != nil {
		return stats, err
	}
	if err := q.db.Table("users").Where("deleted_at IS NULL").Count(&stats.TotalUsers).Error; err != nil {
		return stats, err
	}

	return stats, nil
}
