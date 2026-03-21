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
	Create(product *models.Product) error
	Update(product *models.Product) error
	Delete(id uuid.UUID) error
	CreateImage(img *models.ProductImage) error
	FindImageByID(imageID, productID uuid.UUID) (*models.ProductImage, error)
	DeleteImage(imageID uuid.UUID) error
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

var validStatuses = map[string]bool{
	models.ProductStatusReparado:   true,
	models.ProductStatusEnProgreso: true,
	models.ProductStatusNoReparado: true,
}

type CreateProductRequest struct {
	Name              string           `json:"name" binding:"required,min=1,max=200"`
	RepairDescription string           `json:"repair_description" binding:"max=2000"`
	RepairReference   string           `json:"repair_reference" binding:"max=200"`
	EntryDate         *models.DateOnly `json:"entry_date"`
	ExitDate          *models.DateOnly `json:"exit_date"`
	Observations      string           `json:"observations" binding:"max=2000"`
	Price             *float64         `json:"price" binding:"omitempty,gte=0"`
	CategoryID        *uuid.UUID       `json:"category_id"`
	Paid              *bool            `json:"paid"`
	Status            string           `json:"status"`
}

type UpdateProductRequest struct {
	Name              *string          `json:"name" binding:"omitempty,min=1,max=200"`
	RepairDescription *string          `json:"repair_description" binding:"omitempty,max=2000"`
	RepairReference   *string          `json:"repair_reference" binding:"omitempty,max=200"`
	EntryDate         *models.DateOnly `json:"entry_date"`
	ExitDate          *models.DateOnly `json:"exit_date"`
	Observations      *string          `json:"observations" binding:"omitempty,max=2000"`
	Price             *float64         `json:"price" binding:"omitempty,gte=0"`
	CategoryID        *uuid.UUID       `json:"category_id"`
	Paid              *bool            `json:"paid"`
	Status            *string          `json:"status"`
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
	if s := c.Query("status"); s != "" && validStatuses[s] {
		filter.Status = &s
	}
	if s := c.Query("paid"); s != "" {
		v := s == "true"
		filter.Paid = &v
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

	if h.minioSvc != nil {
		slice := []models.Product{*product}
		h.enrichImageURLs(slice)
		*product = slice[0]
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

	status := models.ProductStatusEnProgreso
	if req.Status != "" {
		if !validStatuses[req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status; allowed: reparado, en_progreso, no_reparado"})
			return
		}
		status = req.Status
	}

	paid := false
	if req.Paid != nil {
		paid = *req.Paid
	}

	price := 0.0
	if req.Price != nil {
		price = *req.Price
	}

	product := &models.Product{
		Name:              req.Name,
		RepairDescription: req.RepairDescription,
		RepairReference:   req.RepairReference,
		EntryDate:         req.EntryDate,
		ExitDate:          req.ExitDate,
		Observations:      req.Observations,
		Price:             price,
		CategoryID:        req.CategoryID,
		CreatedByID:       userID,
		Paid:              paid,
		Status:            status,
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
	if req.RepairDescription != nil {
		product.RepairDescription = *req.RepairDescription
	}
	if req.RepairReference != nil {
		product.RepairReference = *req.RepairReference
	}
	if req.EntryDate != nil {
		product.EntryDate = req.EntryDate
	}
	if req.ExitDate != nil {
		product.ExitDate = req.ExitDate
	}
	if req.Observations != nil {
		product.Observations = *req.Observations
	}
	if req.Price != nil {
		product.Price = *req.Price
	}
	if req.CategoryID != nil {
		product.CategoryID = req.CategoryID
	}
	if req.Paid != nil {
		product.Paid = *req.Paid
	}
	if req.Status != nil {
		if !validStatuses[*req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status; allowed: reparado, en_progreso, no_reparado"})
			return
		}
		product.Status = *req.Status
	}

	if err := h.productRepo.Update(product); err != nil {
		slog.Error("update product", slog.String("id", id.String()), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update product"})
		return
	}

	if h.minioSvc != nil {
		slice := []models.Product{*product}
		h.enrichImageURLs(slice)
		*product = slice[0]
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

	// Managers can only delete products they created
	role, _ := middleware.GetUserRole(c)
	if role == models.RoleManager {
		userID, ok := middleware.GetUserID(c)
		if !ok || product.CreatedByID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "you can only delete products you created"})
			return
		}
	}

	// Delete all images from MinIO (best-effort)
	if h.minioSvc != nil {
		if product.ImageKey != "" {
			h.minioSvc.DeleteObject(product.ImageKey)
		}
		for _, img := range product.Images {
			if img.ImageKey != "" {
				h.minioSvc.DeleteObject(img.ImageKey)
			}
		}
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

// enrichImageURLs populates ImageURL from ImageKey for a slice of products,
// and also populates each ProductImage's ImageURL from its ImageKey.
func (h *ProductHandler) enrichImageURLs(products []models.Product) {
	if h.minioSvc == nil {
		return
	}
	for i := range products {
		if products[i].ImageKey != "" {
			if url, err := h.minioSvc.GetPresignedURL(products[i].ImageKey, time.Hour); err != nil {
				slog.Warn("presign url for list",
					slog.String("key", products[i].ImageKey),
					slog.String("error", err.Error()),
				)
			} else {
				products[i].ImageURL = url
			}
		}
		for j := range products[i].Images {
			if products[i].Images[j].ImageKey == "" {
				continue
			}
			url, err := h.minioSvc.GetPresignedURL(products[i].Images[j].ImageKey, time.Hour)
			if err != nil {
				slog.Warn("presign url for image",
					slog.String("key", products[i].Images[j].ImageKey),
					slog.String("error", err.Error()),
				)
				continue
			}
			products[i].Images[j].ImageURL = url
		}
	}
}

// AddImage uploads a new image and appends it to the product's image gallery.
func (h *ProductHandler) AddImage(c *gin.Context) {
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

	objectKey, err := h.minioSvc.UploadProductImage(file, header)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	position := len(product.Images)
	img := &models.ProductImage{
		ProductID: id,
		ImageKey:  objectKey,
		Position:  position,
	}
	if err := h.productRepo.CreateImage(img); err != nil {
		h.minioSvc.DeleteObject(objectKey)
		slog.Error("create product image", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save image"})
		return
	}

	imageURL, _ := h.minioSvc.GetPresignedURL(objectKey, time.Hour)
	img.ImageURL = imageURL
	c.JSON(http.StatusCreated, img)
}

// DeleteImage removes a single image from the product gallery.
func (h *ProductHandler) DeleteImage(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product ID"})
		return
	}
	imageID, err := uuid.Parse(c.Param("imageId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image ID"})
		return
	}

	img, err := h.productRepo.FindImageByID(imageID, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch image"})
		return
	}

	if h.minioSvc != nil && img.ImageKey != "" {
		h.minioSvc.DeleteObject(img.ImageKey)
	}

	if err := h.productRepo.DeleteImage(imageID); err != nil {
		slog.Error("delete product image", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete image"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "image deleted"})
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
