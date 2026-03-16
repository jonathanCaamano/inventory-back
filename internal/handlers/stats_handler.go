package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StatsHandler struct {
	db *gorm.DB
}

func NewStatsHandler(db *gorm.DB) *StatsHandler {
	return &StatsHandler{db: db}
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
	var stats InventoryStats

	// Single query for all product metrics
	type productAgg struct {
		Total      int64
		Active     int64
		TotalStock int64
		StockValue float64
		OutOfStock int64
		LowStock   int64
	}
	var agg productAgg
	h.db.Raw(`
		SELECT
			COUNT(*)                                        AS total,
			COUNT(*) FILTER (WHERE active = true)           AS active,
			COALESCE(SUM(stock), 0)                         AS total_stock,
			COALESCE(SUM(price * stock), 0)                 AS stock_value,
			COUNT(*) FILTER (WHERE stock = 0)               AS out_of_stock,
			COUNT(*) FILTER (WHERE stock > 0 AND stock <= 5) AS low_stock
		FROM products
		WHERE deleted_at IS NULL
	`).Scan(&agg)

	stats.TotalProducts = agg.Total
	stats.ActiveProducts = agg.Active
	stats.TotalStock = agg.TotalStock
	stats.StockValue = agg.StockValue
	stats.OutOfStock = agg.OutOfStock
	stats.LowStock = agg.LowStock

	h.db.Table("categories").Where("deleted_at IS NULL").Count(&stats.TotalCategories)
	h.db.Table("users").Where("deleted_at IS NULL").Count(&stats.TotalUsers)

	c.JSON(http.StatusOK, stats)
}
