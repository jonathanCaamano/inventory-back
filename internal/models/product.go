package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Product struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	Name        string         `gorm:"not null" json:"name"`
	Description string         `json:"description"`
	Price       float64        `gorm:"not null;default:0" json:"price"`
	Stock       int            `gorm:"not null;default:0" json:"stock"`
	SKU         string         `gorm:"uniqueIndex" json:"sku,omitempty"`
	ImageURL    string         `json:"image_url,omitempty"`
	ImageKey    string         `gorm:"column:image_key" json:"-"` // MinIO object key
	CategoryID  *uuid.UUID     `gorm:"type:uuid" json:"category_id,omitempty"`
	Category    *Category      `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	CreatedByID uuid.UUID      `gorm:"type:uuid;not null" json:"created_by_id"`
	CreatedBy   *User          `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
	Paid        bool           `gorm:"not null;default:false" json:"paid"`
	Active      bool           `gorm:"not null;default:true" json:"active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (p *Product) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
