package models

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const skuChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generateSKU() string {
	b := make([]byte, 6)
	for i := range b {
		b[i] = skuChars[rand.Intn(len(skuChars))]
	}
	return fmt.Sprintf("REP-%s", string(b))
}

// Product status values.
const (
	ProductStatusReparado   = "reparado"
	ProductStatusEnProgreso = "en_progreso"
	ProductStatusNoReparado = "no_reparado"
)

type Product struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	Name        string         `gorm:"not null" json:"name"`
	Description string         `json:"description"`
	Price       float64        `gorm:"not null;default:0" json:"price"`
	SKU         string         `gorm:"uniqueIndex" json:"sku,omitempty"`
	ImageURL    string         `json:"image_url,omitempty"`
	ImageKey    string         `gorm:"column:image_key" json:"-"` // MinIO object key
	CategoryID  *uuid.UUID     `gorm:"type:uuid" json:"category_id,omitempty"`
	Category    *Category      `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	CreatedByID uuid.UUID      `gorm:"type:uuid;not null" json:"created_by_id"`
	CreatedBy   *User          `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
	Contact     *Contact       `gorm:"foreignKey:ProductID" json:"contact,omitempty"`
	Images      []ProductImage `gorm:"foreignKey:ProductID" json:"images,omitempty"`
	Paid        bool           `gorm:"not null;default:false" json:"paid"`
	Status      string         `gorm:"not null;default:'en_progreso'" json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (p *Product) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.SKU == "" {
		// Auto-generate a unique SKU (REP-XXXXXX); retry on collision when DB is available
		if tx != nil {
			for attempts := 0; attempts < 10; attempts++ {
				candidate := generateSKU()
				var count int64
				tx.Model(&Product{}).Where("sku = ?", candidate).Count(&count)
				if count == 0 {
					p.SKU = candidate
					break
				}
			}
		}
		if p.SKU == "" {
			p.SKU = fmt.Sprintf("REP-%s", p.ID.String()[:8])
		}
	}
	return nil
}
