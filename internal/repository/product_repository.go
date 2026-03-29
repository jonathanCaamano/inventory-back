package repository

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"gorm.io/gorm"
)

type ProductFilter struct {
	CategoryID *uuid.UUID
	Search     string
	Status     *string
	Paid       *bool
	Page       int
	PageSize   int
	SortBy     string // entry_date | exit_date | created_at
	SortOrder  string // asc | desc
}

type ProductRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) FindAll(filter ProductFilter) ([]models.Product, int64, error) {
	var products []models.Product
	var total int64

	query := r.db.Model(&models.Product{}).
		Preload("Category").
		Preload("CreatedBy").
		Preload("Contact").
		Preload("Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC, created_at ASC")
		})

	if filter.CategoryID != nil {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if filter.Search != "" {
		query = query.Where("name ILIKE ? OR description ILIKE ? OR sku ILIKE ?",
			"%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.Paid != nil {
		query = query.Where("paid = ?", *filter.Paid)
	}

	query.Count(&total)

	if filter.Page > 0 && filter.PageSize > 0 {
		offset := (filter.Page - 1) * filter.PageSize
		query = query.Offset(offset).Limit(filter.PageSize)
	}

	// Determine sort column and direction
	sortCol := "created_at"
	validCols := map[string]bool{"entry_date": true, "exit_date": true, "created_at": true}
	if filter.SortBy != "" && validCols[filter.SortBy] {
		sortCol = filter.SortBy
	}
	sortDir := "DESC"
	if filter.SortOrder == "asc" {
		sortDir = "ASC"
	}

	var orderExpr string
	if sortCol == "created_at" {
		orderExpr = fmt.Sprintf("created_at %s", sortDir)
	} else {
		// For nullable date columns, always sort NULLs last
		orderExpr = fmt.Sprintf("%s %s NULLS LAST", sortCol, sortDir)
	}

	if err := query.Order(orderExpr).Find(&products).Error; err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

func (r *ProductRepository) FindByID(id uuid.UUID) (*models.Product, error) {
	var product models.Product
	if err := r.db.
		Preload("Category").
		Preload("CreatedBy").
		Preload("Contact").
		Preload("Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC, created_at ASC")
		}).
		First(&product, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *ProductRepository) FindBySKU(sku string) (*models.Product, error) {
	var product models.Product
	if err := r.db.Where("sku = ?", sku).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *ProductRepository) Create(product *models.Product) error {
	return r.db.Create(product).Error
}

func (r *ProductRepository) Update(product *models.Product) error {
	return r.db.Save(product).Error
}

func (r *ProductRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Product{}, "id = ?", id).Error
}

func (r *ProductRepository) CreateImage(img *models.ProductImage) error {
	return r.db.Create(img).Error
}

func (r *ProductRepository) FindImageByID(imageID, productID uuid.UUID) (*models.ProductImage, error) {
	var img models.ProductImage
	if err := r.db.Where("id = ? AND product_id = ?", imageID, productID).First(&img).Error; err != nil {
		return nil, err
	}
	return &img, nil
}

func (r *ProductRepository) DeleteImage(imageID uuid.UUID) error {
	return r.db.Delete(&models.ProductImage{}, "id = ?", imageID).Error
}

type CategoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) FindAll() ([]models.Category, error) {
	var categories []models.Category
	if err := r.db.Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *CategoryRepository) FindByID(id uuid.UUID) (*models.Category, error) {
	var category models.Category
	if err := r.db.First(&category, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *CategoryRepository) Create(category *models.Category) error {
	return r.db.Create(category).Error
}

func (r *CategoryRepository) Update(category *models.Category) error {
	return r.db.Save(category).Error
}

func (r *CategoryRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Category{}, "id = ?", id).Error
}
