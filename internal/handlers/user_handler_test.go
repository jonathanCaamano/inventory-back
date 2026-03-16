package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/models"
)

// ── mocks ────────────────────────────────────────────────────────────────────

type mockUserRepo struct {
	findAll  func() ([]models.User, error)
	findByID func(id uuid.UUID) (*models.User, error)
	create   func(u *models.User) error
	update   func(u *models.User) error
	delete   func(id uuid.UUID) error
}

func (m *mockUserRepo) FindAll() ([]models.User, error) { return m.findAll() }
func (m *mockUserRepo) FindByID(id uuid.UUID) (*models.User, error) {
	if m.findByID != nil {
		return m.findByID(id)
	}
	return nil, errors.New("not found")
}
func (m *mockUserRepo) Create(u *models.User) error {
	if m.create != nil {
		return m.create(u)
	}
	return nil
}
func (m *mockUserRepo) Update(u *models.User) error {
	if m.update != nil {
		return m.update(u)
	}
	return nil
}
func (m *mockUserRepo) Delete(id uuid.UUID) error {
	if m.delete != nil {
		return m.delete(id)
	}
	return nil
}

type mockUserAuthSvc struct {
	hashFn func(pw string) (string, error)
}

func (m *mockUserAuthSvc) HashPassword(pw string) (string, error) {
	if m.hashFn != nil {
		return m.hashFn(pw)
	}
	return "hashed-" + pw, nil
}

func newUserRouter(repo userRepo, svc userAuthService) *gin.Engine {
	r := gin.New()
	h := &UserHandler{userRepo: repo, authSvc: svc}
	r.GET("/users", h.List)
	r.GET("/users/:id", h.Get)
	r.POST("/users", h.Create)
	r.PUT("/users/:id", h.Update)
	r.DELETE("/users/:id", h.Delete)
	return r
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestUserHandler_List_Success(t *testing.T) {
	repo := &mockUserRepo{
		findAll: func() ([]models.User, error) {
			return []models.User{{ID: uuid.New(), Username: "alice"}}, nil
		},
	}
	r := newUserRouter(repo, &mockUserAuthSvc{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/users", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestUserHandler_List_Error(t *testing.T) {
	repo := &mockUserRepo{
		findAll: func() ([]models.User, error) { return nil, errors.New("db error") },
	}
	r := newUserRouter(repo, &mockUserAuthSvc{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/users", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// ── Get ───────────────────────────────────────────────────────────────────────

func TestUserHandler_Get_Success(t *testing.T) {
	id := uuid.New()
	repo := &mockUserRepo{
		findByID: func(_ uuid.UUID) (*models.User, error) {
			return &models.User{ID: id, Username: "bob"}, nil
		},
	}
	r := newUserRouter(repo, &mockUserAuthSvc{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/users/"+id.String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestUserHandler_Get_InvalidUUID(t *testing.T) {
	r := newUserRouter(&mockUserRepo{}, &mockUserAuthSvc{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/users/not-a-uuid", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUserHandler_Get_NotFound(t *testing.T) {
	repo := &mockUserRepo{
		findByID: func(_ uuid.UUID) (*models.User, error) { return nil, errors.New("not found") },
	}
	r := newUserRouter(repo, &mockUserAuthSvc{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/users/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestUserHandler_Create_Success(t *testing.T) {
	r := newUserRouter(&mockUserRepo{}, &mockUserAuthSvc{})

	body, _ := json.Marshal(map[string]string{
		"username": "charlie",
		"email":    "charlie@example.com",
		"password": "password123",
		"role":     "viewer",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUserHandler_Create_BadRequest(t *testing.T) {
	r := newUserRouter(&mockUserRepo{}, &mockUserAuthSvc{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewBufferString("bad json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUserHandler_Create_HashError(t *testing.T) {
	svc := &mockUserAuthSvc{
		hashFn: func(_ string) (string, error) { return "", errors.New("hash error") },
	}
	r := newUserRouter(&mockUserRepo{}, svc)

	body, _ := json.Marshal(map[string]string{
		"username": "dave", "email": "d@x.com", "password": "pass1234", "role": "viewer",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestUserHandler_Create_Conflict(t *testing.T) {
	repo := &mockUserRepo{
		create: func(_ *models.User) error { return errors.New("duplicate key") },
	}
	r := newUserRouter(repo, &mockUserAuthSvc{})

	body, _ := json.Marshal(map[string]string{
		"username": "dup", "email": "dup@x.com", "password": "pass1234", "role": "viewer",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestUserHandler_Update_Success(t *testing.T) {
	id := uuid.New()
	repo := &mockUserRepo{
		findByID: func(_ uuid.UUID) (*models.User, error) {
			return &models.User{ID: id, Username: "old"}, nil
		},
	}
	r := newUserRouter(repo, &mockUserAuthSvc{})

	body, _ := json.Marshal(map[string]string{"username": "new"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/users/"+id.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUserHandler_Update_NotFound(t *testing.T) {
	repo := &mockUserRepo{
		findByID: func(_ uuid.UUID) (*models.User, error) { return nil, errors.New("not found") },
	}
	r := newUserRouter(repo, &mockUserAuthSvc{})

	body, _ := json.Marshal(map[string]string{"username": "new"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/users/"+uuid.New().String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestUserHandler_Delete_Success(t *testing.T) {
	r := newUserRouter(&mockUserRepo{}, &mockUserAuthSvc{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/users/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestUserHandler_Delete_InvalidUUID(t *testing.T) {
	r := newUserRouter(&mockUserRepo{}, &mockUserAuthSvc{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/users/bad-id", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUserHandler_Delete_Error(t *testing.T) {
	repo := &mockUserRepo{
		delete: func(_ uuid.UUID) error { return errors.New("db error") },
	}
	r := newUserRouter(repo, &mockUserAuthSvc{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/users/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
