package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"github.com/jonathanCaamano/inventory-back/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserInactive       = errors.New("user account is inactive")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrWeakPassword       = errors.New("password must be at least 8 characters and contain letters and numbers")
)

type Claims struct {
	UserID   uuid.UUID   `json:"user_id"`
	Username string      `json:"username"`
	Role     models.Role `json:"role"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// userRepository is the subset of UserRepository used by AuthService.
type userRepository interface {
	FindByEmail(email string) (*models.User, error)
	FindByUsername(username string) (*models.User, error)
	FindByID(id uuid.UUID) (*models.User, error)
	UpdateLastLogin(id uuid.UUID) error
}

// tokenRepository is the subset of RefreshTokenRepository used by AuthService.
type tokenRepository interface {
	Create(token *models.RefreshToken) error
	FindByHash(hash string) (*models.RefreshToken, error)
	RevokeByHash(hash string) error
	RevokeAllForUser(userID uuid.UUID) error
}

type AuthService struct {
	userRepo   userRepository
	tokenRepo  tokenRepository
	jwtSecret  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewAuthService(
	userRepo *repository.UserRepository,
	tokenRepo *repository.RefreshTokenRepository,
	jwtSecret string,
	accessTTLHours int,
) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  time.Duration(accessTTLHours) * time.Hour,
		refreshTTL: 30 * 24 * time.Hour, // 30 days
	}
}

// Login authenticates a user and returns an access + refresh token pair.
func (s *AuthService) Login(identifier, password string) (*TokenPair, *models.User, error) {
	user, err := s.userRepo.FindByEmail(identifier)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user, err = s.userRepo.FindByUsername(identifier)
	}
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	if !user.Active {
		return nil, nil, ErrUserInactive
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	pair, err := s.issueTokenPair(user)
	if err != nil {
		return nil, nil, err
	}
	_ = s.userRepo.UpdateLastLogin(user.ID)
	return pair, user, nil
}

// Refresh validates a refresh token and issues a new token pair (rotation).
func (s *AuthService) Refresh(rawToken string) (*TokenPair, *models.User, error) {
	hash := hashToken(rawToken)
	stored, err := s.tokenRepo.FindByHash(hash)
	if err != nil || !stored.IsValid() {
		return nil, nil, ErrInvalidToken
	}

	user, err := s.userRepo.FindByID(stored.UserID)
	if err != nil || !user.Active {
		return nil, nil, ErrInvalidToken
	}

	// Revoke old token (rotation)
	if err := s.tokenRepo.RevokeByHash(hash); err != nil {
		return nil, nil, err
	}

	pair, err := s.issueTokenPair(user)
	if err != nil {
		return nil, nil, err
	}
	return pair, user, nil
}

// Logout revokes the provided refresh token.
func (s *AuthService) Logout(rawToken string) error {
	return s.tokenRepo.RevokeByHash(hashToken(rawToken))
}

// LogoutAll revokes all refresh tokens for a user.
func (s *AuthService) LogoutAll(userID uuid.UUID) error {
	return s.tokenRepo.RevokeAllForUser(userID)
}

// ValidateAccessToken parses and validates a JWT access token.
func (s *AuthService) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func (s *AuthService) CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// HashPassword hashes a plaintext password after validating strength.
func (s *AuthService) HashPassword(password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (s *AuthService) issueTokenPair(user *models.User) (*TokenPair, error) {
	expiresAt := time.Now().Add(s.accessTTL)
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID.String(),
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}

	rawRefresh, err := generateOpaqueToken()
	if err != nil {
		return nil, err
	}
	refreshRecord := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashToken(rawRefresh),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}
	if err := s.tokenRepo.Create(refreshRecord); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    expiresAt,
	}, nil
}

func generateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}
	hasLetter, hasDigit := false, false
	for _, c := range password {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			hasLetter = true
		}
		if c >= '0' && c <= '9' {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrWeakPassword
	}
	return nil
}
