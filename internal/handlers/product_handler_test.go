package handlers

import (
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/middleware"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"github.com/jonathanCaamano/inventory-back/internal/repository"
	"gorm.io/gorm"
)

// ── mocks ────────────────────────────────────────────────────────────────────

type mockProductRepo struct {
	findAllFn   func(filter repository.ProductFilter) ([]models.Product, int64, error)
	findByIDFn  func(id uuid.UUID) (*models.Product, error)
	findBySKUFn func(sku string) (*models.Product, error)
	createFn    func(product *models.Product) error
	updateFn    func(product *models.Product) error
	deleteFn    func(id uuid.UUID) error
}

func (m *mockProductRepo) FindAll(filter repository.ProductFilter) ([]models.Product, int64, error) {
	if m.findAllFn != nil {
		return m.findAllFn(filter)
	}
	return nil, 0, errors.New("not implemented")
}

func (m *mockProductRepo) FindByID(id uuid.UUID) (*models.Product, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(id)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockProductRepo) FindBySKU(sku string) (*models.Product, error) {
	if m.findBySKUFn != nil {
		return m.findBySKUFn(sku)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockProductRepo) Create(product *models.Product) error {
	if m.createFn != nil {
		return m.createFn(product)
	}
	return nil
}

func (m *mockProductRepo) Update(product *models.Product) error {
	if m.updateFn != nil {
		return m.updateFn(product)
	}
	return nil
}

func (m *mockProductRepo) Delete(id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(id)
	}
	return nil
}

type mockCategoryRepo struct {
	findAllFn  func() ([]models.Category, error)
	findByIDFn func(id uuid.UUID) (*models.Category, error)
	createFn   func(category *models.Category) error
	updateFn   func(category *models.Category) error
	deleteFn   func(id uuid.UUID) error
}

func (m *mockCategoryRepo) FindAll() ([]models.Category, error) {
	if m.findAllFn != nil {
		return m.findAllFn()
	}
	return nil, errors.New("not implemented")
}

func (m *mockCategoryRepo) FindByID(id uuid.UUID) (*models.Category, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(id)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockCategoryRepo) Create(category *models.Category) error {
	if m.createFn != nil {
		return m.createFn(category)
	}
	return nil
}

func (m *mockCategoryRepo) Update(category *models.Category) error {
	if m.updateFn != nil {
		return m.updateFn(category)
	}
	return nil
}

func (m *mockCategoryRepo) Delete(id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(id)
	}
	return nil
}

type mockMinioSvc struct {
	getPresignedURLFn    func(key string, expiry time.Duration) (string, error)
	uploadProductImageFn func(file multipart.File, header *multipart.FileHeader) (string, error)
	deleteObjectFn       func(key string)
}

func (m *mockMinioSvc) GetPresignedURL(key string, expiry time.Duration) (string, error) {
	if m.getPresignedURLFn != nil {
		return m.getPresignedURLFn(key, expiry)
	}
	return "http://example.com/image.jpg", nil
}

func (m *mockMinioSvc) UploadProductImage(file multipart.File, header *multipart.FileHeader) (string, error) {
	if m.uploadProductImageFn != nil {
		return m.uploadProductImageFn(file, header)
	}
	return "products/test-key.jpg", nil
}

func (m *mockMinioSvc) DeleteObject(key string) {
	if m.deleteObjectFn != nil {
		m.deleteObjectFn(key)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func fakeProduct() *models.Product {
	return &models.Product{
		ID:     uuid.New(),
		Name:   "Test Product",
		Price:  9.99,
		Stock:  10,
		SKU:    "SKU-001",
		Active: true,
	}
}

func fakeCategory() *models.Category {
	return &models.Category{
		ID:   uuid.New(),
		Name: "Electronics",
	}
}

func newProductRouter(
	productRepo productRepository,
	catRepo categoryRepository,
	minioSvc productMinioService,
) *gin.Engine {
	r := gin.New()
	userID := uuid.New()
	injectUser := func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, userID)
		c.Next()
	}
	h := &ProductHandler{productRepo: productRepo, categoryRepo: catRepo, minioSvc: minioSvc}
	r.GET("/products", h.List)
	r.GET("/products/:id", h.Get)
	r.POST("/products", injectUser, h.Create)
	r.PUT("/products/:id", h.Update)
	r.DELETE("/products/:id", h.Delete)
	r.POST("/products/:id/stock", h.AdjustStock)
	return r
}

func newCategoryRouter(catRepo categoryRepository) *gin.Engine {
	r := gin.New()
	h := &CategoryHandler{categoryRepo: catRepo}
	r.GET("/categories", h.List)
	r.GET("/categories/:id", h.Get)
	r.POST("/categories", h.Create)
	r.PUT("/categories/:id", h.Update)
	r.DELETE("/categories/:id", h.Delete)
	return r
}

// ── Product List ─────────────────────────────────────────────────────────────

func TestProductHandler_List_Success(t *testing.T) {
	products := []models.Product{*fakeProduct(), *fakeProduct()}
	repo := &mockProductRepo{
		findAllFn: func(_ repository.ProductFilter) ([]models.Product, int64, error) {
			return products, 2, nil
		},
	}
	r := newProductRouter(repo, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProductHandler_List_InternalError(t *testing.T) {
	repo := &mockProductRepo{
		findAllFn: func(_ repository.ProductFilter) ([]models.Product, int64, error) {
			return nil, 0, errors.New("db down")
		},
	}
	r := newProductRouter(repo, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// ── Product Get ──────────────────────────────────────────────────────────────

func TestProductHandler_Get_Success(t *testing.T) {
	product := fakeProduct()
	repo := &mockProductRepo{
		findByIDFn: func(_ uuid.UUID) (*models.Product, error) { return product, nil },
	}
	r := newProductRouter(repo, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products/"+product.ID.String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProductHandler_Get_NotFound(t *testing.T) {
	r := newProductRouter(&mockProductRepo{}, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestProductHandler_Get_BadUUID(t *testing.T) {
	r := newProductRouter(&mockProductRepo{}, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products/not-a-uuid", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── Product Create ───────────────────────────────────────────────────────────

func TestProductHandler_Create_Success(t *testing.T) {
	product := fakeProduct()
	repo := &mockProductRepo{
		findBySKUFn: func(_ string) (*models.Product, error) { return nil, gorm.ErrRecordNotFound },
		createFn:    func(_ *models.Product) error { return nil },
		findByIDFn:  func(_ uuid.UUID) (*models.Product, error) { return product, nil },
	}
	r := newProductRouter(repo, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/products",
		jsonBody(t, map[string]any{"name": "Widget", "price": 5.0, "stock": 10, "sku": "WGT-1"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProductHandler_Create_BadRequest(t *testing.T) {
	r := newProductRouter(&mockProductRepo{}, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/products",
		jsonBody(t, map[string]any{"price": 5.0})) // missing required name
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProductHandler_Create_CategoryNotFound(t *testing.T) {
	catID := uuid.New()
	catRepo := &mockCategoryRepo{
		findByIDFn: func(_ uuid.UUID) (*models.Category, error) { return nil, gorm.ErrRecordNotFound },
	}
	r := newProductRouter(&mockProductRepo{}, catRepo, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/products",
		jsonBody(t, map[string]any{"name": "Widget", "category_id": catID.String()}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProductHandler_Create_SKUConflict(t *testing.T) {
	existing := fakeProduct()
	repo := &mockProductRepo{
		findBySKUFn: func(_ string) (*models.Product, error) { return existing, nil },
	}
	r := newProductRouter(repo, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/products",
		jsonBody(t, map[string]any{"name": "Widget", "sku": "EXISTING-SKU"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Product Update ───────────────────────────────────────────────────────────

func TestProductHandler_Update_Success(t *testing.T) {
	product := fakeProduct()
	repo := &mockProductRepo{
		findByIDFn:  func(_ uuid.UUID) (*models.Product, error) { return product, nil },
		findBySKUFn: func(_ string) (*models.Product, error) { return nil, gorm.ErrRecordNotFound },
		updateFn:    func(_ *models.Product) error { return nil },
	}
	r := newProductRouter(repo, &mockCategoryRepo{}, nil)

	newName := "Updated Name"
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/products/"+product.ID.String(),
		jsonBody(t, map[string]any{"name": newName}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProductHandler_Update_NotFound(t *testing.T) {
	r := newProductRouter(&mockProductRepo{}, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/products/"+uuid.New().String(),
		jsonBody(t, map[string]any{"name": "New"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestProductHandler_Update_SKUConflict(t *testing.T) {
	product := fakeProduct()
	product.SKU = "OLD-SKU"
	conflicting := fakeProduct()
	conflicting.SKU = "NEW-SKU"

	repo := &mockProductRepo{
		findByIDFn:  func(_ uuid.UUID) (*models.Product, error) { return product, nil },
		findBySKUFn: func(_ string) (*models.Product, error) { return conflicting, nil },
	}
	r := newProductRouter(repo, &mockCategoryRepo{}, nil)

	newSKU := "NEW-SKU"
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/products/"+product.ID.String(),
		jsonBody(t, map[string]any{"sku": newSKU}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Product Delete ───────────────────────────────────────────────────────────

func TestProductHandler_Delete_Success(t *testing.T) {
	product := fakeProduct()
	repo := &mockProductRepo{
		findByIDFn: func(_ uuid.UUID) (*models.Product, error) { return product, nil },
		deleteFn:   func(_ uuid.UUID) error { return nil },
	}
	r := newProductRouter(repo, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/products/"+product.ID.String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProductHandler_Delete_NotFound(t *testing.T) {
	r := newProductRouter(&mockProductRepo{}, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/products/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── Product AdjustStock ──────────────────────────────────────────────────────

func TestProductHandler_AdjustStock_Success(t *testing.T) {
	product := fakeProduct()
	product.Stock = 10
	repo := &mockProductRepo{
		findByIDFn: func(_ uuid.UUID) (*models.Product, error) { return product, nil },
		updateFn:   func(_ *models.Product) error { return nil },
	}
	r := newProductRouter(repo, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/products/"+product.ID.String()+"/stock",
		jsonBody(t, map[string]any{"delta": 5}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProductHandler_AdjustStock_BadRequest(t *testing.T) {
	r := newProductRouter(&mockProductRepo{}, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/products/"+uuid.New().String()+"/stock",
		jsonBody(t, map[string]any{})) // missing required delta
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProductHandler_AdjustStock_InsufficientStock(t *testing.T) {
	product := fakeProduct()
	product.Stock = 3
	repo := &mockProductRepo{
		findByIDFn: func(_ uuid.UUID) (*models.Product, error) { return product, nil },
	}
	r := newProductRouter(repo, &mockCategoryRepo{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/products/"+product.ID.String()+"/stock",
		jsonBody(t, map[string]any{"delta": -10})) // would result in negative stock
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Category List ─────────────────────────────────────────────────────────────

func TestCategoryHandler_List_Success(t *testing.T) {
	categories := []models.Category{*fakeCategory(), *fakeCategory()}
	catRepo := &mockCategoryRepo{
		findAllFn: func() ([]models.Category, error) { return categories, nil },
	}
	r := newCategoryRouter(catRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/categories", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCategoryHandler_List_InternalError(t *testing.T) {
	catRepo := &mockCategoryRepo{
		findAllFn: func() ([]models.Category, error) { return nil, errors.New("db down") },
	}
	r := newCategoryRouter(catRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/categories", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// ── Category Get ──────────────────────────────────────────────────────────────

func TestCategoryHandler_Get_Success(t *testing.T) {
	cat := fakeCategory()
	catRepo := &mockCategoryRepo{
		findByIDFn: func(_ uuid.UUID) (*models.Category, error) { return cat, nil },
	}
	r := newCategoryRouter(catRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/categories/"+cat.ID.String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCategoryHandler_Get_NotFound(t *testing.T) {
	r := newCategoryRouter(&mockCategoryRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/categories/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCategoryHandler_Get_BadUUID(t *testing.T) {
	r := newCategoryRouter(&mockCategoryRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/categories/not-a-uuid", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── Category Create ───────────────────────────────────────────────────────────

func TestCategoryHandler_Create_Success(t *testing.T) {
	catRepo := &mockCategoryRepo{
		createFn: func(_ *models.Category) error { return nil },
	}
	r := newCategoryRouter(catRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/categories",
		jsonBody(t, map[string]any{"name": "Electronics"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCategoryHandler_Create_BadRequest(t *testing.T) {
	r := newCategoryRouter(&mockCategoryRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/categories",
		jsonBody(t, map[string]any{})) // missing required name
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCategoryHandler_Create_Conflict(t *testing.T) {
	catRepo := &mockCategoryRepo{
		createFn: func(_ *models.Category) error { return errors.New("duplicate key") },
	}
	r := newCategoryRouter(catRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/categories",
		jsonBody(t, map[string]any{"name": "Electronics"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Category Update ───────────────────────────────────────────────────────────

func TestCategoryHandler_Update_Success(t *testing.T) {
	cat := fakeCategory()
	catRepo := &mockCategoryRepo{
		findByIDFn: func(_ uuid.UUID) (*models.Category, error) { return cat, nil },
		updateFn:   func(_ *models.Category) error { return nil },
	}
	r := newCategoryRouter(catRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/categories/"+cat.ID.String(),
		jsonBody(t, map[string]any{"name": "Updated"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCategoryHandler_Update_NotFound(t *testing.T) {
	r := newCategoryRouter(&mockCategoryRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/categories/"+uuid.New().String(),
		jsonBody(t, map[string]any{"name": "Updated"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── Category Delete ───────────────────────────────────────────────────────────

func TestCategoryHandler_Delete_Success(t *testing.T) {
	cat := fakeCategory()
	catRepo := &mockCategoryRepo{
		findByIDFn: func(_ uuid.UUID) (*models.Category, error) { return cat, nil },
		deleteFn:   func(_ uuid.UUID) error { return nil },
	}
	r := newCategoryRouter(catRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/categories/"+cat.ID.String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCategoryHandler_Delete_NotFound(t *testing.T) {
	r := newCategoryRouter(&mockCategoryRepo{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/categories/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
