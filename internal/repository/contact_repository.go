package repository

import (
	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ContactRepository struct {
	db *gorm.DB
}

func NewContactRepository(db *gorm.DB) *ContactRepository {
	return &ContactRepository{db: db}
}

func (r *ContactRepository) FindByProductID(productID uuid.UUID) (*models.Contact, error) {
	var contact models.Contact
	if err := r.db.Where("product_id = ?", productID).First(&contact).Error; err != nil {
		return nil, err
	}
	return &contact, nil
}

// Upsert creates or updates the contact for a given product (one contact per product).
func (r *ContactRepository) Upsert(contact *models.Contact) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "product_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "subdato", "email", "phone", "updated_at"}),
	}).Create(contact).Error
}

func (r *ContactRepository) Delete(productID uuid.UUID) error {
	return r.db.Where("product_id = ?", productID).Delete(&models.Contact{}).Error
}
