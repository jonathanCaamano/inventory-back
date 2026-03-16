package handlers

import (
	"errors"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/middleware"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"github.com/jonathanCaamano/inventory-back/internal/repository"
	"github.com/jonathanCaamano/inventory-back/internal/services"
	"gorm.io/gorm"
)

type productRepository interface {
	FindAll(filter repository.ProductFilter) ([]models.Product, int64, error)
	FindByID(id uuid.UUID) (*models.Product, error)
	FindBySKU(sku string) (*models.Product, error)
	Create(product *models.Product) error
	Update(product *models.Product) error
	Delete(id uuid.UUID) error
}

type categoryRepository interface {
	FindAll() ([]models.Category, error)
	FindByID(id uuid.UUID) (*models.Category, error)
	Create(category *models.Category) error
	Update(category *models.Category) error
	Delete(id uuid.UUID) error
}

type productMinioService interface {
	GetPresignedURL(objectKey string, expiry time.Duration) (string, error)
	UploadProductImage(file multipart.File, header *multipart.FileHeader) (string, error)
	DeleteObject(objectKey string)
}

type ProductHandler struct {
	productRepo  productRepository
	categoryRepo categoryRepository
	minioSvc     productMinioService
}

func NewProductHandler(
	productRepo *repository.ProductRepository,
	categoryRepo *repository.CategoryRepository,
	minioSvc *services.MinIOService,
) *ProductHandler {
	return &ProductHandler{
		productRepo:  productRepo,
		categoryRepo: categoryRepo,
		minioSvc:     minioSvc,
	}
}

type CreateProductRequest struct {
	Name        string     `json:"name" binding:"required,min=1,max=200"`
	Description string     `json:"description" binding:"max=2000"`
	Price       float64    `json:"price" binding:"gte=0"`
	Stock       int        `json:"stock" binding:"gte=0"`
	SKU         string     `json:"sku" binding:"omitempty,max=100"`
	CategoryID  *uuid.UUID `json:"category_id"`
	Active      *bool      `json:"active"`
}

type UpdateProductRequest struct {
	Name        *string    `json:"name" binding:"omitempty,min=1,max=200"`
	Description *string    `json:"description" binding:"omitempty,max=2000"`
	Price       *float64   `json:"price" binding:"omitempty,gte=0"`
	Stock       *int       `json:"stock" binding:"omitempty,gte=0"`
	SKU         *string    `json:"sku" binding:"omitempty,max=100"`
	CategoryID  *uuid.UUID `json:"category_id"`
	Active      *bool      `json:"active"`
}

type StockAdjustRequest struct {
	Delta int    `json:"delta" binding:"required"`
	Note  string `json:"note" binding:"max=200"`
}

func (h *ProductHandler) List(c *gin.Context) {
	filter := repository.ProductFilter{Page: 1, PageSize: 20}

	filter.Search = c.Query("search")
	if page, err := strconv.Atoi(c.Query("page")); err == nil && page > 0 {
		filter.Page = page
	}
	if size, err := strconv.Atoi(c.Query("page_size")); err == nil && size > 0 && size <= 100 {
		filter.PageSize = size
	}
	if catID, err := uuid.Parse(c.Query("category_id")); err == nil {
		filter.CategoryID = &catID
	}
	if s := c.Query("active"); s != "" {
		v := s == "true"
		filter.Active = &v
	}

	products, total, err := h.productRepo.FindAll(filter)
	if err != nil {
		slog.Error("list products", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch products"})
		return
	}

	h.enrichImageURLs(products)

	c.JSON(http.StatusOK, gin.H{
		"data":      products,
		"total":     total,
		"page":      filter.Page,
		"page_size": filter.PageSize,
	})
}

func (h *ProductHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product ID"})
		return
	}
	product, err := h.productRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		slog.Error("get product", slog.String("id", id.String()), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch product"})
		return
	}

	if product.ImageKey != "" && h.minioSvc != nil {
		if url, err := h.minioSvc.GetPresignedURL(product.ImageKey, time.Hour); err != nil {
			slog.Warn("presign url", slog.String("key", product.ImageKey), slog.String("error", err.Error()))
		} else {
			product.ImageURL = url
		}
	}

	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) Create(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Validate category exists
	if req.CategoryID != nil {
		if _, err := h.categoryRepo.FindByID(*req.CategoryID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category not found"})
			return
		}
	}

	// Check SKU uniqueness
	if req.SKU != "" {
		if _, err := h.productRepo.FindBySKU(req.SKU); err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "SKU already exists"})
			return
		}
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	product := &models.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		SKU:         req.SKU,
		CategoryID:  req.CategoryID,
		CreatedByID: userID,
		Active:      active,
	}

	if err := h.productRepo.Create(product); err != nil {
		slog.Error("create product", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create product"})
		return
	}

	// Reload with associations; log but don't fail
	if loaded, err := h.productRepo.FindByID(product.ID); err != nil {
		slog.Warn("reload after create failed", slog.String("id", product.ID.String()), slog.String("error", err.Error()))
		c.JSON(http.StatusCreated, product)
	} else {
		c.JSON(http.StatusCreated, loaded)
	}
}

func (h *ProductHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product ID"})
		return
	}

	product, err := h.productRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch product"})
		return
	}

	var req UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// SKU uniqueness check (only if changing to a different SKU)
	if req.SKU != nil && *req.SKU != "" && *req.SKU != product.SKU {
		if existing, err := h.productRepo.FindBySKU(*req.SKU); err == nil && existing.ID != product.ID {
			c.JSON(http.StatusConflict, gin.H{"error": "SKU already in use"})
			return
		}
	}

	// Category existence check
	if req.CategoryID != nil {
		if _, err := h.categoryRepo.FindByID(*req.CategoryID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category not found"})
			return
		}
	}

	if req.Name != nil {
		product.Name = *req.Name
	}
	if req.Description != nil {
		product.Description = *req.Description
	}
	if req.Price != nil {
		product.Price = *req.Price
	}
	if req.Stock != nil {
		product.Stock = *req.Stock
	}
	if req.SKU != nil {
		product.SKU = *req.SKU
	}
	if req.CategoryID != nil {
		product.CategoryID = req.CategoryID
	}
	if req.Active != nil {
		product.Active = *req.Active
	}

	if err := h.productRepo.Update(product); err != nil {
		slog.Error("update product", slog.String("id", id.String()), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update product"})
		return
	}

	if product.ImageKey != "" && h.minioSvc != nil {
		if url, err := h.minioSvc.GetPresignedURL(product.ImageKey, time.Hour); err != nil {
			slog.Warn("presign url", slog.String("key", product.ImageKey), slog.String("error", err.Error()))
		} else {
			product.ImageURL = url
		}
	}
	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product ID"})
		return
	}

	product, err := h.productRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch product"})
		return
	}

	// Delete image from MinIO (best-effort, errors are logged inside DeleteObject)
	if product.ImageKey != "" && h.minioSvc != nil {
		h.minioSvc.DeleteObject(product.ImageKey)
	}

	if err := h.productRepo.Delete(id); err != nil {
		slog.Error("delete product", slog.String("id", id.String()), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete product"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "product deleted"})
}

func (h *ProductHandler) UploadImage(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product ID"})
		return
	}

	product, err := h.productRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch product"})
		return
	}

	if h.minioSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "image storage not available"})
		return
	}

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file required"})
		return
	}
	defer func() { _ = file.Close() }()

	// Delete old image (best-effort)
	if product.ImageKey != "" {
		h.minioSvc.DeleteObject(product.ImageKey)
	}

	objectKey, err := h.minioSvc.UploadProductImage(file, header)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product.ImageKey = objectKey
	if err := h.productRepo.Update(product); err != nil {
		// Rollback the upload since we can't persist the reference
		h.minioSvc.DeleteObject(objectKey)
		slog.Error("update product image key", slog.String("id", id.String()), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update product image"})
		return
	}

	imageURL, _ := h.minioSvc.GetPresignedURL(objectKey, time.Hour)
	c.JSON(http.StatusOK, gin.H{"image_url": imageURL})
}

// AdjustStock increments or decrements stock by a delta value.
func (h *ProductHandler) AdjustStock(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product ID"})
		return
	}

	var req StockAdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product, err := h.productRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch product"})
		return
	}

	newStock := product.Stock + req.Delta
	if newStock < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient stock"})
		return
	}
	product.Stock = newStock

	if err := h.productRepo.Update(product); err != nil {
		slog.Error("adjust stock", slog.String("id", id.String()), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update stock"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stock": product.Stock})
}

// enrichImageURLs populates ImageURL from ImageKey for a slice of products.
func (h *ProductHandler) enrichImageURLs(products []models.Product) {
	if h.minioSvc == nil {
		return
	}
	for i := range products {
		if products[i].ImageKey == "" {
			continue
		}
		url, err := h.minioSvc.GetPresignedURL(products[i].ImageKey, time.Hour)
		if err != nil {
			slog.Warn("presign url for list",
				slog.String("key", products[i].ImageKey),
				slog.String("error", err.Error()),
			)
			continue
		}
		products[i].ImageURL = url
	}
}

// ---------------------------------------------------------------------------
// Category Handler
// ---------------------------------------------------------------------------

type CategoryHandler struct {
	categoryRepo categoryRepository
}

func NewCategoryHandler(categoryRepo *repository.CategoryRepository) *CategoryHandler {
	return &CategoryHandler{categoryRepo: categoryRepo}
}

type CreateCategoryRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=100"`
	Description string `json:"description" binding:"max=500"`
}

func (h *CategoryHandler) List(c *gin.Context) {
	categories, err := h.categoryRepo.FindAll()
	if err != nil {
		slog.Error("list categories", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch categories"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": categories, "total": len(categories)})
}

func (h *CategoryHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category ID"})
		return
	}
	category, err := h.categoryRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch category"})
		return
	}
	c.JSON(http.StatusOK, category)
}

func (h *CategoryHandler) Create(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	category := &models.Category{Name: req.Name, Description: req.Description}
	if err := h.categoryRepo.Create(category); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "category name already exists"})
		return
	}
	c.JSON(http.StatusCreated, category)
}

func (h *CategoryHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category ID"})
		return
	}
	category, err := h.categoryRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch category"})
		return
	}
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	category.Name = req.Name
	category.Description = req.Description
	if err := h.categoryRepo.Update(category); err != nil {
		slog.Error("update category", slog.String("id", id.String()), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update category"})
		return
	}
	c.JSON(http.StatusOK, category)
}

func (h *CategoryHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category ID"})
		return
	}
	if _, err := h.categoryRepo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}
	}
	if err := h.categoryRepo.Delete(id); err != nil {
		slog.Error("delete category", slog.String("id", id.String()), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete category"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "category deleted"})
}
