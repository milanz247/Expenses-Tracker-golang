package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"expense-tracker-api/models"
)

type BudgetHandler struct {
	db *gorm.DB
}

func NewBudgetHandler(db *gorm.DB) *BudgetHandler {
	return &BudgetHandler{db: db}
}

// POST /api/budgets
func (h *BudgetHandler) UpsertBudget(c *gin.Context) {
	userId, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userId.(uint)

	var req models.UpsertBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify category belongs to user
	var category models.Category
	if err := h.db.Where("id = ? AND user_id = ?", req.CategoryID, userID).First(&category).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	// Upsert budget (update if exists, else insert)
	var budget models.Budget
	result := h.db.Where("user_id = ? AND category_id = ? AND month = ? AND year = ?",
		userID, req.CategoryID, req.Month, req.Year).First(&budget)

	if result.Error == gorm.ErrRecordNotFound {
		budget = models.Budget{
			UserID:     userID,
			CategoryID: req.CategoryID,
			Amount:     req.Amount,
			Month:      req.Month,
			Year:       req.Year,
		}
		if err := h.db.Create(&budget).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create budget"})
			return
		}
	} else if result.Error == nil {
		budget.Amount = req.Amount
		if err := h.db.Save(&budget).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update budget"})
			return
		}
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, budget)
}

// GET /api/budgets
func (h *BudgetHandler) GetBudgets(c *gin.Context) {
	userId, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userId.(uint)

	monthStr := c.Query("month")
	yearStr := c.Query("year")

	query := h.db.Where("user_id = ?", userID)
	if monthStr != "" {
		query = query.Where("month = ?", monthStr)
	}
	if yearStr != "" {
		query = query.Where("year = ?", yearStr)
	}

	var budgets []models.Budget
	if err := query.Find(&budgets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch budgets"})
		return
	}

	// Enrich with category name and spending
	type BudgetResponse struct {
		models.Budget
		CategoryName string  `json:"category_name"`
		Spent        float64 `json:"spent"`
		PercentUsed  float64 `json:"percent_used"`
	}

	results := make([]BudgetResponse, len(budgets))
	for i, b := range budgets {
		var cat models.Category
		h.db.First(&cat, b.CategoryID)

		var spent float64
		now := time.Now()
		startOfMonth := time.Date(b.Year, time.Month(b.Month), 1, 0, 0, 0, 0, now.Location())
		endOfMonth := startOfMonth.AddDate(0, 1, 0)
		h.db.Model(&models.Transaction{}).
			Select("COALESCE(SUM(amount), 0)").
			Where("user_id = ? AND category = ? AND type = 'expense' AND date >= ? AND date < ? AND deleted_at IS NULL",
				userID, cat.Name, startOfMonth, endOfMonth).
			Scan(&spent)

		percentUsed := float64(0)
		if b.Amount > 0 {
			percentUsed = (spent / b.Amount) * 100
		}

		results[i] = BudgetResponse{
			Budget:       b,
			CategoryName: cat.Name,
			Spent:        spent,
			PercentUsed:  percentUsed,
		}
	}

	c.JSON(http.StatusOK, gin.H{"budgets": results})
}

// DELETE /api/budgets/:id
func (h *BudgetHandler) DeleteBudget(c *gin.Context) {
	userId, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userId.(uint)

	id := c.Param("id")
	var budget models.Budget
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&budget).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Budget not found"})
		return
	}

	if err := h.db.Delete(&budget).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete budget"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Budget deleted"})
}
