package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/middleware"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"github.com/jonathanCaamano/inventory-back/internal/services"
)

// ── mocks ────────────────────────────────────────────────────────────────────

type mockAuthSvc struct {
	loginFn         func(id, pw string) (*services.TokenPair, *models.User, error)
	refreshFn       func(tok string) (*services.TokenPair, *models.User, error)
	logoutFn        func(tok string) error
	logoutAllFn     func(id uuid.UUID) error
	hashPasswordFn  func(pw string) (string, error)
	checkPasswordFn func(hash, plain string) bool
}

func (m *mockAuthSvc) Login(id, pw string) (*services.TokenPair, *models.User, error) {
	return m.loginFn(id, pw)
}
func (m *mockAuthSvc) Refresh(tok string) (*services.TokenPair, *models.User, error) {
	return m.refreshFn(tok)
}
func (m *mockAuthSvc) Logout(tok string) error      { return m.logoutFn(tok) }
func (m *mockAuthSvc) LogoutAll(id uuid.UUID) error { return m.logoutAllFn(id) }
func (m *mockAuthSvc) HashPassword(pw string) (string, error) {
	if m.hashPasswordFn != nil {
		return m.hashPasswordFn(pw)
	}
	return pw, nil
}
func (m *mockAuthSvc) CheckPassword(hash, plain string) bool {
	if m.checkPasswordFn != nil {
		return m.checkPasswordFn(hash, plain)
	}
	return hash == plain
}

type mockAuthUserRepo struct {
	findByIDFn func(id uuid.UUID) (*models.User, error)
	createFn   func(user *models.User) error
	updateFn   func(user *models.User) error
}

func (m *mockAuthUserRepo) FindByID(id uuid.UUID) (*models.User, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(id)
	}
	return nil, errors.New("not implemented")
}
func (m *mockAuthUserRepo) Create(user *models.User) error {
	if m.createFn != nil {
		return m.createFn(user)
	}
	return nil
}
func (m *mockAuthUserRepo) Update(user *models.User) error {
	if m.updateFn != nil {
		return m.updateFn(user)
	}
	return nil
}

func fakePair() *services.TokenPair {
	return &services.TokenPair{
		AccessToken:  "access-tok",
		RefreshToken: "refresh-tok",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
}

func fakeUser() *models.User {
	return &models.User{
		ID:       uuid.New(),
		Username: "alice",
		Email:    "alice@example.com",
		Role:     models.RoleViewer,
		Active:   true,
	}
}

func newAuthRouter(svc authService, repo authUserRepo) *gin.Engine {
	r := gin.New()
	h := &AuthHandler{authSvc: svc, userRepo: repo}
	r.POST("/login", h.Login)
	r.POST("/refresh", h.Refresh)
	r.POST("/logout", h.Logout)
	r.POST("/logout-all", func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, fakeUser().ID)
		c.Next()
	}, h.LogoutAll)
	r.GET("/me", func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, fakeUser().ID)
		c.Next()
	}, h.Me)
	return r
}

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// ── Login ────────────────────────────────────────────────────────────────────

func TestAuthHandler_Login_Success(t *testing.T) {
	svc := &mockAuthSvc{
		loginFn: func(_, _ string) (*services.TokenPair, *models.User, error) {
			return fakePair(), fakeUser(), nil
		},
	}
	r := newAuthRouter(svc, &mockAuthUserRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/login",
		jsonBody(t, map[string]string{"identifier": "alice", "password": "pass1234"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthHandler_Login_BadRequest(t *testing.T) {
	r := newAuthRouter(&mockAuthSvc{}, &mockAuthUserRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBufferString("bad json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	svc := &mockAuthSvc{
		loginFn: func(_, _ string) (*services.TokenPair, *models.User, error) {
			return nil, nil, services.ErrInvalidCredentials
		},
	}
	r := newAuthRouter(svc, &mockAuthUserRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/login",
		jsonBody(t, map[string]string{"identifier": "x", "password": "y"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthHandler_Login_InactiveUser(t *testing.T) {
	svc := &mockAuthSvc{
		loginFn: func(_, _ string) (*services.TokenPair, *models.User, error) {
			return nil, nil, services.ErrUserInactive
		},
	}
	r := newAuthRouter(svc, &mockAuthUserRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/login",
		jsonBody(t, map[string]string{"identifier": "x", "password": "y"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestAuthHandler_Login_InternalError(t *testing.T) {
	svc := &mockAuthSvc{
		loginFn: func(_, _ string) (*services.TokenPair, *models.User, error) {
			return nil, nil, errors.New("db down")
		},
	}
	r := newAuthRouter(svc, &mockAuthUserRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/login",
		jsonBody(t, map[string]string{"identifier": "x", "password": "y"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestAuthHandler_Refresh_Success(t *testing.T) {
	svc := &mockAuthSvc{
		refreshFn: func(_ string) (*services.TokenPair, *models.User, error) {
			return fakePair(), fakeUser(), nil
		},
	}
	r := newAuthRouter(svc, &mockAuthUserRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/refresh",
		jsonBody(t, map[string]string{"refresh_token": "some-token"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthHandler_Refresh_Invalid(t *testing.T) {
	svc := &mockAuthSvc{
		refreshFn: func(_ string) (*services.TokenPair, *models.User, error) {
			return nil, nil, services.ErrInvalidToken
		},
	}
	r := newAuthRouter(svc, &mockAuthUserRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/refresh",
		jsonBody(t, map[string]string{"refresh_token": "bad"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// ── Logout ────────────────────────────────────────────────────────────────────

func TestAuthHandler_Logout_WithToken(t *testing.T) {
	revoked := ""
	svc := &mockAuthSvc{
		logoutFn: func(tok string) error { revoked = tok; return nil },
	}
	r := newAuthRouter(svc, &mockAuthUserRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/logout",
		jsonBody(t, map[string]string{"refresh_token": "my-token"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if revoked == "" {
		t.Error("expected token to be revoked")
	}
}

func TestAuthHandler_Logout_NoToken(t *testing.T) {
	svc := &mockAuthSvc{
		logoutFn: func(_ string) error { return nil },
	}
	r := newAuthRouter(svc, &mockAuthUserRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/logout", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ── LogoutAll ─────────────────────────────────────────────────────────────────

func TestAuthHandler_LogoutAll_Success(t *testing.T) {
	svc := &mockAuthSvc{
		logoutAllFn: func(_ uuid.UUID) error { return nil },
	}
	r := newAuthRouter(svc, &mockAuthUserRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/logout-all", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthHandler_LogoutAll_Error(t *testing.T) {
	svc := &mockAuthSvc{
		logoutAllFn: func(_ uuid.UUID) error { return errors.New("db error") },
	}
	r := newAuthRouter(svc, &mockAuthUserRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/logout-all", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// ── Me ────────────────────────────────────────────────────────────────────────

func TestAuthHandler_Me_Success(t *testing.T) {
	user := fakeUser()
	repo := &mockAuthUserRepo{
		findByIDFn: func(_ uuid.UUID) (*models.User, error) { return user, nil },
	}
	r := newAuthRouter(&mockAuthSvc{}, repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/me", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthHandler_Me_NotFound(t *testing.T) {
	repo := &mockAuthUserRepo{
		findByIDFn: func(_ uuid.UUID) (*models.User, error) {
			return nil, errors.New("not found")
		},
	}
	r := newAuthRouter(&mockAuthSvc{}, repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/me", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
