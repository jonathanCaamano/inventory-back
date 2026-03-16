package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Contact struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	ProductID uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex" json:"product_id"`
	Name      string         `gorm:"not null" json:"name"`
	Subdato   string         `gorm:"not null" json:"subdato"`
	Email     string         `json:"email,omitempty"`
	Phone     string         `json:"phone,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (c *Contact) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
