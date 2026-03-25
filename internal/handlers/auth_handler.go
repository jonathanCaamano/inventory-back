package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/middleware"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"github.com/jonathanCaamano/inventory-back/internal/repository"
	"github.com/jonathanCaamano/inventory-back/internal/services"
)

type authService interface {
	Login(identifier, password string) (*services.TokenPair, *models.User, error)
	Refresh(rawToken string) (*services.TokenPair, *models.User, error)
	Logout(rawToken string) error
	LogoutAll(userID uuid.UUID) error
	HashPassword(password string) (string, error)
	CheckPassword(hash, plain string) bool
}

type authUserRepo interface {
	FindByID(id uuid.UUID) (*models.User, error)
	Create(user *models.User) error
	Update(user *models.User) error
}

type AuthHandler struct {
	authSvc  authService
	userRepo authUserRepo
}

func NewAuthHandler(authSvc *services.AuthService, userRepo *repository.UserRepository) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, userRepo: userRepo}
}

type loginRequest struct {
	Identifier string `json:"identifier" binding:"required"`
	Password   string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type registerRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type updateMeRequest struct {
	Username        string `json:"username" binding:"omitempty,min=3,max=50"`
	Email           string `json:"email" binding:"omitempty,email"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password" binding:"omitempty,min=8"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, user, err := h.authSvc.Login(req.Identifier, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		case errors.Is(err, services.ErrUserInactive):
			c.JSON(http.StatusForbidden, gin.H{"error": "account is inactive"})
		default:
			slog.Error("login error", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_at":    pair.ExpiresAt,
		"user":          user,
	})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, user, err := h.authSvc.Refresh(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_at":    pair.ExpiresAt,
		"user":          user,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req logoutRequest
	_ = c.ShouldBindJSON(&req) // optional body

	if req.RefreshToken != "" {
		if err := h.authSvc.Logout(req.RefreshToken); err != nil {
			slog.Warn("logout revoke error", slog.String("error", err.Error()))
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.authSvc.LogoutAll(userID); err != nil {
		slog.Error("logout all error", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke sessions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "all sessions revoked"})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := h.authSvc.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         models.RoleViewer,
		Active:       true,
	}

	if err := h.userRepo.Create(user); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username or email already exists"})
		return
	}

	pair, _, err := h.authSvc.Login(req.Username, req.Password)
	if err != nil {
		slog.Error("register auto-login error", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "registration succeeded but login failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_at":    pair.ExpiresAt,
		"user":          user,
	})
}

func (h *AuthHandler) UpdateMe(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.NewPassword != "" {
		if req.CurrentPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "current_password is required to change password"})
			return
		}
		if !h.authSvc.CheckPassword(user.PasswordHash, req.CurrentPassword) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
			return
		}
		hash, err := h.authSvc.HashPassword(req.NewPassword)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		user.PasswordHash = hash
	}

	if err := h.userRepo.Update(user); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username or email already in use"})
		return
	}
	c.JSON(http.StatusOK, user)
}
