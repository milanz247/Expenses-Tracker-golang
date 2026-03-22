package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"expense-tracker-api/models"
)

type CategoryHandler struct {
	db *gorm.DB
}

func NewCategoryHandler(db *gorm.DB) *CategoryHandler {
	return &CategoryHandler{db: db}
}

// POST /api/categories
func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var req models.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Prevent duplicate name+type per user
	var existing models.Category
	if err := h.db.Where("user_id = ? AND name = ? AND type = ?", userID, req.Name, req.Type).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Category already exists"})
		return
	}

	category := models.Category{
		UserID: userID,
		Name:   req.Name,
		Type:   req.Type,
	}

	if err := h.db.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}

	c.JSON(http.StatusCreated, category)
}

// GET /api/categories
func (h *CategoryHandler) GetCategories(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var categories []models.Category
	if err := h.db.Where("user_id = ?", userID).Order("type, name").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// DELETE /api/categories/:id
func (h *CategoryHandler) DeleteCategory(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var category models.Category
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), userID).First(&category).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	if err := h.db.Delete(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete category"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Category deleted"})
}
