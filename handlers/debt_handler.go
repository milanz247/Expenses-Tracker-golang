package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"expense-tracker-api/models"
)

type DebtHandler struct {
	db *gorm.DB
}

func NewDebtHandler(db *gorm.DB) *DebtHandler {
	return &DebtHandler{db: db}
}

// POST /api/debts
func (h *DebtHandler) CreateDebt(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var req models.CreateDebtRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var debt models.Debt

	err := h.db.Transaction(func(tx *gorm.DB) error {
		// Lock and fetch account
		var account models.Account
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", req.AccountID, userID).First(&account).Error; err != nil {
			return fmt.Errorf("account not found")
		}

		// LEND: money goes out (balance decreases) — needs balance check
		// BORROW: money comes in (balance increases)
		switch req.Type {
		case "LEND":
			if req.Amount > account.Balance {
				return fmt.Errorf("Insufficient balance")
			}
			if err := tx.Model(&account).Update("balance", gorm.Expr("balance - ?", req.Amount)).Error; err != nil {
				return err
			}
		case "BORROW":
			if err := tx.Model(&account).Update("balance", gorm.Expr("balance + ?", req.Amount)).Error; err != nil {
				return err
			}
		}

		// Create auto-transaction
		txType := "expense"
		category := "Lending"
		desc := fmt.Sprintf("Lent to %s", req.PersonName)
		if req.Type == "BORROW" {
			txType = "income"
			category = "Borrowing"
			desc = fmt.Sprintf("Borrowed from %s", req.PersonName)
		}
		if req.Description != "" {
			desc = req.Description
		}

		txn := models.Transaction{
			UserID:      userID,
			AccountID:   req.AccountID,
			Amount:      req.Amount,
			Type:        txType,
			Category:    category,
			Description: desc,
		}
		if err := tx.Create(&txn).Error; err != nil {
			return err
		}

		debt = models.Debt{
			UserID:        userID,
			AccountID:     req.AccountID,
			PersonName:    req.PersonName,
			Description:   req.Description,
			Amount:        req.Amount,
			PaidAmount:    0,
			Type:          req.Type,
			DueDate:       req.DueDate,
			Status:        "OPEN",
			TransactionID: &txn.ID,
		}
		return tx.Create(&debt).Error
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, debt)
}

// GET /api/debts
func (h *DebtHandler) GetDebts(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	db := h.db.Where("debts.user_id = ?", userID)

	if t := c.Query("type"); t != "" {
		db = db.Where("debts.type = ?", t)
	}
	if s := c.Query("status"); s != "" {
		db = db.Where("debts.status = ?", s)
	}

	type DebtRow struct {
		models.Debt
		AccountName string `json:"account_name"`
	}

	var rows []DebtRow
	err := db.Table("debts").
		Select("debts.*, accounts.name as account_name").
		Joins("LEFT JOIN accounts ON accounts.id = debts.account_id").
		Where("debts.deleted_at IS NULL").
		Order("debts.created_at DESC").
		Scan(&rows).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch debts"})
		return
	}

	if rows == nil {
		rows = []DebtRow{}
	}

	c.JSON(http.StatusOK, gin.H{"debts": rows})
}

// POST /api/debts/:id/repay
func (h *DebtHandler) RepayDebt(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	debtID := c.Param("id")

	var req models.RepayDebtRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		var debt models.Debt
		if err := tx.Where("id = ? AND user_id = ?", debtID, userID).First(&debt).Error; err != nil {
			return fmt.Errorf("debt not found")
		}

		if debt.Status == "CLOSED" {
			return fmt.Errorf("this debt is already closed")
		}

		remaining := debt.Amount - debt.PaidAmount
		repayAmount := req.Amount
		if repayAmount > remaining {
			repayAmount = remaining
		}

		// Lock and fetch repay account
		var account models.Account
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", req.AccountID, userID).First(&account).Error; err != nil {
			return fmt.Errorf("account not found")
		}

		// LEND repayment: money comes back (balance increases)
		// BORROW repayment: money goes out (balance decreases) — needs balance check
		switch debt.Type {
		case "LEND":
			if err := tx.Model(&account).Update("balance", gorm.Expr("balance + ?", repayAmount)).Error; err != nil {
				return err
			}
		case "BORROW":
			if repayAmount > account.Balance {
				return fmt.Errorf("Insufficient balance")
			}
			if err := tx.Model(&account).Update("balance", gorm.Expr("balance - ?", repayAmount)).Error; err != nil {
				return err
			}
		}

		// Create repayment transaction
		txType := "income"
		category := "Debt Repayment"
		desc := fmt.Sprintf("Repayment from %s", debt.PersonName)
		if debt.Type == "BORROW" {
			txType = "expense"
			desc = fmt.Sprintf("Repayment to %s", debt.PersonName)
		}

		txn := models.Transaction{
			UserID:      userID,
			AccountID:   req.AccountID,
			Amount:      repayAmount,
			Type:        txType,
			Category:    category,
			Description: desc,
			DebtID:      &debt.ID,
		}
		if err := tx.Create(&txn).Error; err != nil {
			return err
		}

		// Update debt paid amount
		newPaid := debt.PaidAmount + repayAmount
		updates := map[string]interface{}{"paid_amount": newPaid}
		if newPaid >= debt.Amount {
			updates["status"] = "CLOSED"
		}

		return tx.Model(&debt).Updates(updates).Error
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Reload and return updated debt
	var updated models.Debt
	h.db.First(&updated, debtID)
	c.JSON(http.StatusOK, updated)
}

// GET /api/debts/summary
func (h *DebtHandler) GetDebtSummary(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var toPay float64
	h.db.Raw("SELECT COALESCE(SUM(amount - paid_amount), 0) FROM debts WHERE user_id = ? AND type = 'BORROW' AND status = 'OPEN' AND deleted_at IS NULL", userID).
		Scan(&toPay)

	var toReceive float64
	h.db.Raw("SELECT COALESCE(SUM(amount - paid_amount), 0) FROM debts WHERE user_id = ? AND type = 'LEND' AND status = 'OPEN' AND deleted_at IS NULL", userID).
		Scan(&toReceive)

	c.JSON(http.StatusOK, gin.H{
		"to_pay":     toPay,
		"to_receive": toReceive,
	})
}

// DELETE /api/debts/:id
func (h *DebtHandler) DeleteDebt(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	debtID := c.Param("id")

	err := h.db.Transaction(func(tx *gorm.DB) error {
		// Fetch and lock the debt
		var debt models.Debt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", debtID, userID).First(&debt).Error; err != nil {
			return fmt.Errorf("debt not found")
		}

		// ── 1. Reverse and delete all repayment transactions ──────────────────
		var repayTxns []models.Transaction
		if err := tx.Where("debt_id = ? AND user_id = ?", debt.ID, userID).Find(&repayTxns).Error; err != nil {
			return fmt.Errorf("failed to fetch repayment transactions")
		}

		for _, repayTxn := range repayTxns {
			var repayAcct models.Account
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND user_id = ?", repayTxn.AccountID, userID).First(&repayAcct).Error; err != nil {
				return fmt.Errorf("repayment account not found")
			}

			// LEND repayment was income (balance+) → reversal: balance-
			// BORROW repayment was expense (balance-) → reversal: balance+
			switch debt.Type {
			case "LEND":
				if repayAcct.Balance < repayTxn.Amount {
					return fmt.Errorf("Cannot delete this debt because repayment funds have already been spent from account '%s'. Please delete related expenses first.", repayAcct.Name)
				}
				if err := tx.Model(&repayAcct).Update("balance", gorm.Expr("balance - ?", repayTxn.Amount)).Error; err != nil {
					return err
				}
			case "BORROW":
				if err := tx.Model(&repayAcct).Update("balance", gorm.Expr("balance + ?", repayTxn.Amount)).Error; err != nil {
					return err
				}
			}

			if err := tx.Unscoped().Delete(&repayTxn).Error; err != nil {
				return err
			}
		}

		// ── 2. Reverse and delete the initial transaction ──────────────────────
		var initTxn models.Transaction
		var txFound bool

		if debt.TransactionID != nil {
			if err := tx.Where("id = ? AND user_id = ?", *debt.TransactionID, userID).First(&initTxn).Error; err == nil {
				txFound = true
			}
		} else {
			// Fallback for legacy debts without transaction_id
			category := "Lending"
			if debt.Type == "BORROW" {
				category = "Borrowing"
			}
			if err := tx.Where("account_id = ? AND amount = ? AND category = ? AND user_id = ?", debt.AccountID, debt.Amount, category, userID).First(&initTxn).Error; err == nil {
				txFound = true
			}
		}

		if txFound {
			var initAcct models.Account
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND user_id = ?", initTxn.AccountID, userID).First(&initAcct).Error; err != nil {
				return fmt.Errorf("original account not found")
			}

			// LEND initial was expense (balance-) → reversal: balance+
			// BORROW initial was income (balance+) → reversal: balance-
			switch debt.Type {
			case "LEND":
				if err := tx.Model(&initAcct).Update("balance", gorm.Expr("balance + ?", initTxn.Amount)).Error; err != nil {
					return err
				}
			case "BORROW":
				if initAcct.Balance < initTxn.Amount {
					return fmt.Errorf("Cannot delete this debt because the borrowed funds have already been spent from account '%s'. Please delete related expenses first.", initAcct.Name)
				}
				if err := tx.Model(&initAcct).Update("balance", gorm.Expr("balance - ?", initTxn.Amount)).Error; err != nil {
					return err
				}
			}

			if err := tx.Unscoped().Delete(&initTxn).Error; err != nil {
				return err
			}
		}

		// ── 3. Hard-delete the debt record itself ──────────────────────────────
		return tx.Unscoped().Delete(&debt).Error
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Debt and all associated transactions deleted and balances reversed"})
}
