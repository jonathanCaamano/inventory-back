package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ── mock interfaces ──────────────────────────────────────────────────────────

type mockUserRepo struct {
	findByEmail    func(email string) (*models.User, error)
	findByUsername func(username string) (*models.User, error)
	findByID       func(id uuid.UUID) (*models.User, error)
	updateLogin    func(id uuid.UUID) error
}

func (m *mockUserRepo) FindByEmail(email string) (*models.User, error) {
	return m.findByEmail(email)
}
func (m *mockUserRepo) FindByUsername(username string) (*models.User, error) {
	return m.findByUsername(username)
}
func (m *mockUserRepo) FindByID(id uuid.UUID) (*models.User, error) {
	return m.findByID(id)
}
func (m *mockUserRepo) UpdateLastLogin(id uuid.UUID) error {
	if m.updateLogin != nil {
		return m.updateLogin(id)
	}
	return nil
}

type mockTokenRepo struct {
	create           func(t *models.RefreshToken) error
	findByHash       func(hash string) (*models.RefreshToken, error)
	revokeByHash     func(hash string) error
	revokeAllForUser func(id uuid.UUID) error
}

func (m *mockTokenRepo) Create(t *models.RefreshToken) error {
	if m.create != nil {
		return m.create(t)
	}
	return nil
}
func (m *mockTokenRepo) FindByHash(hash string) (*models.RefreshToken, error) {
	return m.findByHash(hash)
}
func (m *mockTokenRepo) RevokeByHash(hash string) error {
	if m.revokeByHash != nil {
		return m.revokeByHash(hash)
	}
	return nil
}
func (m *mockTokenRepo) RevokeAllForUser(id uuid.UUID) error {
	if m.revokeAllForUser != nil {
		return m.revokeAllForUser(id)
	}
	return nil
}

// ── helper ───────────────────────────────────────────────────────────────────

func newTestService(ur userRepository, tr tokenRepository) *AuthService {
	return &AuthService{
		userRepo:   ur,
		tokenRepo:  tr,
		jwtSecret:  []byte("test-secret-key-that-is-32-chars!!"),
		accessTTL:  time.Hour,
		refreshTTL: 30 * 24 * time.Hour,
	}
}

func hashPass(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

// ── Login ────────────────────────────────────────────────────────────────────

func TestLogin_ByEmail_Success(t *testing.T) {
	userID := uuid.New()
	user := &models.User{
		ID:           userID,
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: hashPass(t, "password123"),
		Role:         models.RoleViewer,
		Active:       true,
	}
	ur := &mockUserRepo{
		findByEmail:    func(_ string) (*models.User, error) { return user, nil },
		findByUsername: func(_ string) (*models.User, error) { return nil, gorm.ErrRecordNotFound },
	}
	tr := &mockTokenRepo{}
	svc := newTestService(ur, tr)

	pair, got, err := svc.Login("alice@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Error("expected non-empty tokens")
	}
	if got.ID != userID {
		t.Errorf("expected user ID %s, got %s", userID, got.ID)
	}
}

func TestLogin_ByUsername_FallbackSuccess(t *testing.T) {
	user := &models.User{
		ID:           uuid.New(),
		Username:     "bob",
		Email:        "bob@example.com",
		PasswordHash: hashPass(t, "password123"),
		Role:         models.RoleManager,
		Active:       true,
	}
	ur := &mockUserRepo{
		findByEmail:    func(_ string) (*models.User, error) { return nil, gorm.ErrRecordNotFound },
		findByUsername: func(_ string) (*models.User, error) { return user, nil },
	}
	svc := newTestService(ur, &mockTokenRepo{})

	_, got, err := svc.Login("bob", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Username != "bob" {
		t.Errorf("expected username bob, got %s", got.Username)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	ur := &mockUserRepo{
		findByEmail:    func(_ string) (*models.User, error) { return nil, gorm.ErrRecordNotFound },
		findByUsername: func(_ string) (*models.User, error) { return nil, gorm.ErrRecordNotFound },
	}
	svc := newTestService(ur, &mockTokenRepo{})

	_, _, err := svc.Login("nobody@example.com", "password123")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	user := &models.User{
		ID:           uuid.New(),
		PasswordHash: hashPass(t, "correct-password1"),
		Active:       true,
	}
	ur := &mockUserRepo{
		findByEmail:    func(_ string) (*models.User, error) { return user, nil },
		findByUsername: func(_ string) (*models.User, error) { return nil, gorm.ErrRecordNotFound },
	}
	svc := newTestService(ur, &mockTokenRepo{})

	_, _, err := svc.Login("user@example.com", "wrong-password1")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_InactiveUser(t *testing.T) {
	user := &models.User{
		ID:           uuid.New(),
		PasswordHash: hashPass(t, "password123"),
		Active:       false,
	}
	ur := &mockUserRepo{
		findByEmail:    func(_ string) (*models.User, error) { return user, nil },
		findByUsername: func(_ string) (*models.User, error) { return nil, gorm.ErrRecordNotFound },
	}
	svc := newTestService(ur, &mockTokenRepo{})

	_, _, err := svc.Login("user@example.com", "password123")
	if !errors.Is(err, ErrUserInactive) {
		t.Errorf("expected ErrUserInactive, got %v", err)
	}
}

// ── ValidateAccessToken ───────────────────────────────────────────────────────

func TestValidateAccessToken_Valid(t *testing.T) {
	user := &models.User{
		ID:       uuid.New(),
		Username: "charlie",
		Role:     models.RoleAdmin,
		Active:   true,
	}
	ur := &mockUserRepo{
		findByEmail:    func(_ string) (*models.User, error) { return user, nil },
		findByUsername: func(_ string) (*models.User, error) { return nil, gorm.ErrRecordNotFound },
	}
	svc := newTestService(ur, &mockTokenRepo{})

	pair, _, err := svc.Login("charlie@example.com", "")
	// Login will fail on bcrypt but let's test token generation directly
	_ = pair
	_ = err

	// Issue a token directly via issueTokenPair
	tp, err := svc.issueTokenPair(user)
	if err != nil {
		t.Fatalf("issueTokenPair failed: %v", err)
	}

	claims, err := svc.ValidateAccessToken(tp.AccessToken)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if claims.UserID != user.ID {
		t.Errorf("expected userID %s, got %s", user.ID, claims.UserID)
	}
	if claims.Role != models.RoleAdmin {
		t.Errorf("expected role admin, got %s", claims.Role)
	}
}

func TestValidateAccessToken_Invalid(t *testing.T) {
	svc := newTestService(&mockUserRepo{}, &mockTokenRepo{})

	_, err := svc.ValidateAccessToken("not.a.valid.token")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidateAccessToken_WrongSecret(t *testing.T) {
	user := &models.User{ID: uuid.New(), Username: "dave", Role: models.RoleViewer}
	svc1 := newTestService(&mockUserRepo{}, &mockTokenRepo{})
	svc2 := &AuthService{
		jwtSecret: []byte("completely-different-secret-key!!"),
		accessTTL: time.Hour,
	}

	tp, _ := svc1.issueTokenPair(user)
	_, err := svc2.ValidateAccessToken(tp.AccessToken)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestRefresh_Success(t *testing.T) {
	userID := uuid.New()
	user := &models.User{ID: userID, Username: "eve", Role: models.RoleViewer, Active: true}
	rawToken := "raw-opaque-token-value"
	storedToken := &models.RefreshToken{
		UserID:    userID,
		TokenHash: hashToken(rawToken),
		ExpiresAt: time.Now().Add(time.Hour),
		Revoked:   false,
	}

	ur := &mockUserRepo{
		findByID: func(_ uuid.UUID) (*models.User, error) { return user, nil },
	}
	tr := &mockTokenRepo{
		findByHash: func(_ string) (*models.RefreshToken, error) { return storedToken, nil },
	}
	svc := newTestService(ur, tr)

	pair, got, err := svc.Refresh(rawToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pair.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if got.ID != userID {
		t.Errorf("expected user ID %s", userID)
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	tr := &mockTokenRepo{
		findByHash: func(_ string) (*models.RefreshToken, error) { return nil, errors.New("not found") },
	}
	svc := newTestService(&mockUserRepo{}, tr)

	_, _, err := svc.Refresh("bad-token")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestRefresh_RevokedToken(t *testing.T) {
	storedToken := &models.RefreshToken{
		UserID:    uuid.New(),
		ExpiresAt: time.Now().Add(time.Hour),
		Revoked:   true,
	}
	tr := &mockTokenRepo{
		findByHash: func(_ string) (*models.RefreshToken, error) { return storedToken, nil },
	}
	svc := newTestService(&mockUserRepo{}, tr)

	_, _, err := svc.Refresh("some-token")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestRefresh_ExpiredToken(t *testing.T) {
	storedToken := &models.RefreshToken{
		UserID:    uuid.New(),
		ExpiresAt: time.Now().Add(-time.Hour),
		Revoked:   false,
	}
	tr := &mockTokenRepo{
		findByHash: func(_ string) (*models.RefreshToken, error) { return storedToken, nil },
	}
	svc := newTestService(&mockUserRepo{}, tr)

	_, _, err := svc.Refresh("some-token")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

// ── Logout ────────────────────────────────────────────────────────────────────

func TestLogout(t *testing.T) {
	revoked := ""
	tr := &mockTokenRepo{
		revokeByHash: func(hash string) error { revoked = hash; return nil },
	}
	svc := newTestService(&mockUserRepo{}, tr)

	err := svc.Logout("my-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if revoked == "" {
		t.Error("expected token to be revoked")
	}
}

func TestLogoutAll(t *testing.T) {
	userID := uuid.New()
	called := false
	tr := &mockTokenRepo{
		revokeAllForUser: func(id uuid.UUID) error {
			if id != userID {
				return errors.New("wrong user ID")
			}
			called = true
			return nil
		},
	}
	svc := newTestService(&mockUserRepo{}, tr)

	if err := svc.LogoutAll(userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected RevokeAllForUser to be called")
	}
}

// ── HashPassword ──────────────────────────────────────────────────────────────

func TestHashPassword_Valid(t *testing.T) {
	svc := newTestService(&mockUserRepo{}, &mockTokenRepo{})

	hash, err := svc.HashPassword("validPass1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("validPass1")); err != nil {
		t.Error("hash does not match original password")
	}
}

func TestHashPassword_TooShort(t *testing.T) {
	svc := newTestService(&mockUserRepo{}, &mockTokenRepo{})

	_, err := svc.HashPassword("abc1")
	if !errors.Is(err, ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got %v", err)
	}
}

func TestHashPassword_NoDigit(t *testing.T) {
	svc := newTestService(&mockUserRepo{}, &mockTokenRepo{})

	_, err := svc.HashPassword("onlyletters")
	if !errors.Is(err, ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got %v", err)
	}
}

func TestHashPassword_NoLetter(t *testing.T) {
	svc := newTestService(&mockUserRepo{}, &mockTokenRepo{})

	_, err := svc.HashPassword("12345678")
	if !errors.Is(err, ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got %v", err)
	}
}

// ── validatePassword ──────────────────────────────────────────────────────────

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		password string
		wantErr  bool
	}{
		{"validPass1", false},
		{"Abcdefg1", false},
		{"short1", true}, // < 8 chars
		{"noDIGIT!", true},
		{"12345678", true},
		{"", true},
	}

	for _, tt := range tests {
		err := validatePassword(tt.password)
		if (err != nil) != tt.wantErr {
			t.Errorf("validatePassword(%q): got err=%v, wantErr=%v", tt.password, err, tt.wantErr)
		}
	}
}
