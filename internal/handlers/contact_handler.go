package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"github.com/jonathanCaamano/inventory-back/internal/repository"
	"gorm.io/gorm"
)

type contactRepo interface {
	FindByProductID(productID uuid.UUID) (*models.Contact, error)
	Upsert(contact *models.Contact) error
	Delete(productID uuid.UUID) error
}

type contactProductRepo interface {
	FindByID(id uuid.UUID) (*models.Product, error)
}

type ContactHandler struct {
	contactRepo contactRepo
	productRepo contactProductRepo
}

func NewContactHandler(
	contactRepo *repository.ContactRepository,
	productRepo *repository.ProductRepository,
) *ContactHandler {
	return &ContactHandler{contactRepo: contactRepo, productRepo: productRepo}
}

type UpsertContactRequest struct {
	Name    string `json:"name"    binding:"required,min=1,max=200"`
	Subdato string `json:"subdato" binding:"required,min=1,max=200"`
	Email   string `json:"email"   binding:"omitempty,email"`
	Phone   string `json:"phone"   binding:"omitempty,max=50"`
}

func (h *ContactHandler) Get(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product ID"})
		return
	}

	contact, err := h.contactRepo.FindByProductID(productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch contact"})
		return
	}
	c.JSON(http.StatusOK, contact)
}

func (h *ContactHandler) Upsert(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product ID"})
		return
	}

	if _, err := h.productRepo.FindByID(productID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch product"})
		return
	}

	var req UpsertContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contact := &models.Contact{
		ProductID: productID,
		Name:      req.Name,
		Subdato:   req.Subdato,
		Email:     req.Email,
		Phone:     req.Phone,
	}

	if err := h.contactRepo.Upsert(contact); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save contact"})
		return
	}

	// Return the persisted record
	saved, _ := h.contactRepo.FindByProductID(productID)
	if saved == nil {
		saved = contact
	}
	c.JSON(http.StatusOK, saved)
}

func (h *ContactHandler) Delete(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product ID"})
		return
	}

	if err := h.contactRepo.Delete(productID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete contact"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "contact deleted"})
}
