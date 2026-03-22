package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
		// Lock and fetch source account
		var src models.Account
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", req.AccountID, userID).First(&src).Error; err != nil {
			return fmt.Errorf("source account not found")
		}

		switch req.Type {
		case "income":
			if err := tx.Model(&src).Update("balance", gorm.Expr("balance + ?", req.Amount)).Error; err != nil {
				return err
			}
		case "expense":
			if req.Amount > src.Balance {
				return fmt.Errorf("Insufficient balance")
			}
			if err := tx.Model(&src).Update("balance", gorm.Expr("balance - ?", req.Amount)).Error; err != nil {
				return err
			}
		case "transfer":
			if req.Amount > src.Balance {
				return fmt.Errorf("Insufficient balance")
			}
			var dst models.Account
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", *req.ToAccountID, userID).First(&dst).Error; err != nil {
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
		ID          uint      `json:"id"`
		AccountID   uint      `json:"account_id"`
		AccountName string    `json:"account_name"`
		ToAccountID *uint     `json:"to_account_id,omitempty"`
		Amount      float64   `json:"amount"`
		Type        string    `json:"type"`
		Category    string    `json:"category"`
		Description string    `json:"description"`
		Date        time.Time `json:"date"`
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

// DELETE /api/transactions/:id
func (h *AccountHandler) DeleteTransaction(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction ID"})
		return
	}

	var resultMsg string

	err = h.db.Transaction(func(tx *gorm.DB) error {
		// Fetch the transaction (verify ownership)
		var txn models.Transaction
		if err := tx.Where("id = ? AND user_id = ?", id, userID).First(&txn).Error; err != nil {
			return fmt.Errorf("transaction not found")
		}

		// ─── CASE A: Initial Debt Transaction (Lending / Borrowing) ───────────
		if txn.Category == "Lending" || txn.Category == "Borrowing" {
			// Find the linked debt record
			var linkedDebt models.Debt
			found := false

			if err := tx.Where("transaction_id = ? AND user_id = ?", txn.ID, userID).First(&linkedDebt).Error; err == nil {
				found = true
			} else {
				// Fallback for legacy debts without transaction_id
				debtType := "LEND"
				if txn.Category == "Borrowing" {
					debtType = "BORROW"
				}
				if err := tx.Where("account_id = ? AND amount = ? AND type = ? AND user_id = ? AND transaction_id IS NULL",
					txn.AccountID, txn.Amount, debtType, userID).First(&linkedDebt).Error; err == nil {
					found = true
				}
			}

			if found {
				// 1. Find and reverse ALL repayment transactions for this debt
				var repayTxns []models.Transaction
				if err := tx.Where("debt_id = ? AND user_id = ?", linkedDebt.ID, userID).Find(&repayTxns).Error; err != nil {
					return fmt.Errorf("failed to fetch repayment transactions")
				}

				for _, repayTxn := range repayTxns {
					var repayAcct models.Account
					if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
						Where("id = ? AND user_id = ?", repayTxn.AccountID, userID).First(&repayAcct).Error; err != nil {
						return fmt.Errorf("repayment account not found")
					}

					// LEND repayment was income → reverse: subtract
					// BORROW repayment was expense → reverse: add back
					switch linkedDebt.Type {
					case "LEND":
						if repayAcct.Balance < repayTxn.Amount {
							return fmt.Errorf("Cannot delete: repayment funds already spent from account '%s'. Delete related expenses first.", repayAcct.Name)
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

				// 2. Reverse the initial transaction's balance impact
				var src models.Account
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("id = ? AND user_id = ?", txn.AccountID, userID).First(&src).Error; err != nil {
					return fmt.Errorf("source account not found")
				}

				switch linkedDebt.Type {
				case "LEND":
					// Lend was expense (balance-) → reverse: add back
					if err := tx.Model(&src).Update("balance", gorm.Expr("balance + ?", txn.Amount)).Error; err != nil {
						return err
					}
				case "BORROW":
					// Borrow was income (balance+) → reverse: subtract
					if src.Balance < txn.Amount {
						return fmt.Errorf("Cannot delete: the borrowed funds have already been spent from account '%s'. Delete related expenses first.", src.Name)
					}
					if err := tx.Model(&src).Update("balance", gorm.Expr("balance - ?", txn.Amount)).Error; err != nil {
						return err
					}
				}

				// 3. Delete the debt record
				if err := tx.Unscoped().Delete(&linkedDebt).Error; err != nil {
					return err
				}

				// 4. Delete the initial transaction
				if err := tx.Unscoped().Delete(&txn).Error; err != nil {
					return err
				}

				resultMsg = "Debt record, all repayment history, and transaction deleted. Balances reversed."
				return nil
			}
		}

		// ─── CASE B: Repayment Transaction (Debt Repayment) ──────────────────
		if txn.Category == "Debt Repayment" && txn.DebtID != nil {
			var linkedDebt models.Debt
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND user_id = ?", *txn.DebtID, userID).First(&linkedDebt).Error; err != nil {
				return fmt.Errorf("linked debt not found")
			}

			// 1. Update debt totals: PaidAmount -= repayment amount
			newPaid := linkedDebt.PaidAmount - txn.Amount
			if newPaid < 0 {
				newPaid = 0
			}
			updates := map[string]interface{}{"paid_amount": newPaid}
			// If it was CLOSED (fully settled), reopen it
			if linkedDebt.Status == "CLOSED" {
				updates["status"] = "OPEN"
			}
			if err := tx.Model(&linkedDebt).Updates(updates).Error; err != nil {
				return err
			}

			// 2. Reverse balance impact
			var src models.Account
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND user_id = ?", txn.AccountID, userID).First(&src).Error; err != nil {
				return fmt.Errorf("source account not found")
			}

			switch linkedDebt.Type {
			case "LEND":
				// LEND repayment was income (balance+) → reverse: subtract
				if src.Balance < txn.Amount {
					return fmt.Errorf("Cannot delete this repayment because the funds have already been spent. Please delete related expenses first.")
				}
				if err := tx.Model(&src).Update("balance", gorm.Expr("balance - ?", txn.Amount)).Error; err != nil {
					return err
				}
			case "BORROW":
				// BORROW repayment was expense (balance-) → reverse: add back
				if err := tx.Model(&src).Update("balance", gorm.Expr("balance + ?", txn.Amount)).Error; err != nil {
					return err
				}
			}

			// 3. Delete only this repayment transaction
			if err := tx.Unscoped().Delete(&txn).Error; err != nil {
				return err
			}

			resultMsg = "Repayment deleted. Debt status updated and balance reversed."
			return nil
		}

		// ─── CASE C: Regular Transaction (no debt link) ──────────────────────
		var src models.Account
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", txn.AccountID, userID).First(&src).Error; err != nil {
			return fmt.Errorf("source account not found")
		}

		switch txn.Type {
		case "expense":
			if err := tx.Model(&src).Update("balance", gorm.Expr("balance + ?", txn.Amount)).Error; err != nil {
				return err
			}
		case "income":
			if src.Balance < txn.Amount {
				return fmt.Errorf("Cannot delete this transaction because the funds have already been spent or the account balance would become negative. Please delete the related expenses first.")
			}
			if err := tx.Model(&src).Update("balance", gorm.Expr("balance - ?", txn.Amount)).Error; err != nil {
				return err
			}
		case "transfer":
			if txn.ToAccountID != nil {
				var dst models.Account
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", *txn.ToAccountID, userID).First(&dst).Error; err != nil {
					return fmt.Errorf("destination account not found")
				}
				if dst.Balance < txn.Amount {
					return fmt.Errorf("Cannot delete this transfer because the destination account has already used the funds. Please delete the related expenses first.")
				}
				if err := tx.Model(&dst).Update("balance", gorm.Expr("balance - ?", txn.Amount)).Error; err != nil {
					return err
				}
			}
			if err := tx.Model(&src).Update("balance", gorm.Expr("balance + ?", txn.Amount)).Error; err != nil {
				return err
			}
		}

		if err := tx.Unscoped().Delete(&txn).Error; err != nil {
			return err
		}

		resultMsg = "Transaction reversed and deleted successfully"
		return nil
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": resultMsg})
}
