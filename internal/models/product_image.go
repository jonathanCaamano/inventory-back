package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProductImage struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	ProductID uuid.UUID      `gorm:"type:uuid;not null;index" json:"product_id"`
	ImageURL  string         `json:"image_url,omitempty"`
	ImageKey  string         `gorm:"column:image_key" json:"-"`
	Position  int            `gorm:"not null;default:0" json:"position"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (pi *ProductImage) BeforeCreate(tx *gorm.DB) error {
	if pi.ID == uuid.Nil {
		pi.ID = uuid.New()
	}
	return nil
}
