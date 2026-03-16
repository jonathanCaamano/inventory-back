package repository

import (
	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"gorm.io/gorm"
)

type RefreshTokenRepository struct {
	db *gorm.DB
}

func NewRefreshTokenRepository(db *gorm.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(token *models.RefreshToken) error {
	return r.db.Create(token).Error
}

func (r *RefreshTokenRepository) FindByHash(hash string) (*models.RefreshToken, error) {
	var t models.RefreshToken
	if err := r.db.Where("token_hash = ? AND revoked = false", hash).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *RefreshTokenRepository) RevokeByHash(hash string) error {
	return r.db.Model(&models.RefreshToken{}).
		Where("token_hash = ?", hash).
		Update("revoked", true).Error
}

func (r *RefreshTokenRepository) RevokeAllForUser(userID uuid.UUID) error {
	return r.db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked = false", userID).
		Update("revoked", true).Error
}

// PurgeExpired deletes expired and revoked tokens. Call periodically.
func (r *RefreshTokenRepository) PurgeExpired() error {
	return r.db.Unscoped().
		Where("revoked = true OR expires_at < NOW()").
		Delete(&models.RefreshToken{}).Error
}
