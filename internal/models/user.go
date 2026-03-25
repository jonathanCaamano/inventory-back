package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleManager Role = "manager"
	RoleViewer  Role = "viewer"
)

type User struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	Username     string         `gorm:"uniqueIndex;not null" json:"username"`
	Email        string         `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"not null" json:"-"`
	Role         Role           `gorm:"type:varchar(20);not null;default:'viewer'" json:"role"`
	Active       bool           `gorm:"not null;default:true" json:"active"`
	LastLogin    *time.Time     `json:"last_login,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (u *User) CanManage() bool {
	return u.Role == RoleAdmin || u.Role == RoleManager
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}
