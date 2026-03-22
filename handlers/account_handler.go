package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"expense-tracker-api/models"
)

type AccountHandler struct {
	db *gorm.DB
}

func NewAccountHandler(db *gorm.DB) *AccountHandler {
	return &AccountHandler{db: db}
}

// POST /api/accounts
func (h *AccountHandler) CreateAccount(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var req models.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	account := models.Account{
		UserID:  userID,
		Name:    req.Name,
		Type:    req.Type,
		Balance: req.Balance,
	}

	if err := h.db.Create(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}

	c.JSON(http.StatusCreated, account)
}

// GET /api/accounts
func (h *AccountHandler) GetAccounts(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var accounts []models.Account
	if err := h.db.Where("user_id = ?", userID).Find(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch accounts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"accounts": accounts})
}

// POST /api/transactions
func (h *AccountHandler) CreateTransaction(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var req models.CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Type == "transfer" && req.ToAccountID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "to_account_id is required for transfers"})
		return
	}

	var txn models.Transaction

	err := h.db.Transaction(func(tx *gorm.DB) error {
		// Verify source account belongs to user
		var src models.Account
		if err := tx.Where("id = ? AND user_id = ?", req.AccountID, userID).First(&src).Error; err != nil {
			return fmt.Errorf("source account not found")
		}

		switch req.Type {
		case "income":
			if err := tx.Model(&src).Update("balance", gorm.Expr("balance + ?", req.Amount)).Error; err != nil {
				return err
			}
		case "expense":
			if err := tx.Model(&src).Update("balance", gorm.Expr("balance - ?", req.Amount)).Error; err != nil {
				return err
			}
		case "transfer":
			var dst models.Account
			if err := tx.Where("id = ? AND user_id = ?", *req.ToAccountID, userID).First(&dst).Error; err != nil {
				return fmt.Errorf("destination account not found")
			}
			if err := tx.Model(&src).Update("balance", gorm.Expr("balance - ?", req.Amount)).Error; err != nil {
				return err
			}
			if err := tx.Model(&dst).Update("balance", gorm.Expr("balance + ?", req.Amount)).Error; err != nil {
				return err
			}
		}

		date := req.Date
		if date.IsZero() {
			date = time.Now()
		}

		txn = models.Transaction{
			UserID:      userID,
			AccountID:   req.AccountID,
			ToAccountID: req.ToAccountID,
			Amount:      req.Amount,
			Type:        req.Type,
			Category:    req.Category,
			Description: req.Description,
			Date:        date,
		}

		return tx.Create(&txn).Error
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, txn)
}

// GET /api/transactions
func (h *AccountHandler) GetTransactions(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	db := h.db.Where("transactions.user_id = ?", userID)

	if t := c.Query("type"); t != "" {
		db = db.Where("transactions.type = ?", t)
	}
	if aid := c.Query("account_id"); aid != "" {
		db = db.Where("transactions.account_id = ?", aid)
	}
	if cat := c.Query("category"); cat != "" {
		db = db.Where("LOWER(transactions.category) LIKE ?", "%"+cat+"%")
	}
	if start := c.Query("start_date"); start != "" {
		db = db.Where("transactions.date >= ?", start)
	}
	if end := c.Query("end_date"); end != "" {
		db = db.Where("transactions.date <= ?", end)
	}

	type TxRow struct {
		ID          uint       `json:"id"`
		AccountID   uint       `json:"account_id"`
		AccountName string     `json:"account_name"`
		ToAccountID *uint      `json:"to_account_id,omitempty"`
		Amount      float64    `json:"amount"`
		Type        string     `json:"type"`
		Category    string     `json:"category"`
		Description string     `json:"description"`
		Date        time.Time  `json:"date"`
	}

	var rows []TxRow
	err := db.Table("transactions").
		Select("transactions.id, transactions.account_id, accounts.name as account_name, transactions.to_account_id, transactions.amount, transactions.type, transactions.category, transactions.description, transactions.date").
		Joins("LEFT JOIN accounts ON accounts.id = transactions.account_id").
		Where("transactions.deleted_at IS NULL").
		Order("transactions.date DESC").
		Scan(&rows).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch transactions"})
		return
	}

	if rows == nil {
		rows = []TxRow{}
	}

	c.JSON(http.StatusOK, gin.H{"transactions": rows})
}
