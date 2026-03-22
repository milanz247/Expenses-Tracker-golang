package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"expense-tracker-api/models"
)

type SummaryHandler struct {
	db *gorm.DB
}

func NewSummaryHandler(db *gorm.DB) *SummaryHandler {
	return &SummaryHandler{db: db}
}

type CategoryBreakdown struct {
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
}

type RecentTransaction struct {
	ID          uint      `json:"id"`
	Date        time.Time `json:"date"`
	AccountName string    `json:"account_name"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Amount      float64   `json:"amount"`
	Type        string    `json:"type"`
}

type DailySpending struct {
	Day    string  `json:"day"`
	Amount float64 `json:"amount"`
}

func (h *SummaryHandler) GetSummary(c *gin.Context) {
	userID := c.GetUint("userID")
	now := time.Now()

	// ── 1. Total Balance ────────────────────────────────────────────────────
	var totalBalance float64
	h.db.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE user_id = ? AND deleted_at IS NULL", userID).
		Scan(&totalBalance)

	// ── 2 & 3. Monthly Income / Expenses ────────────────────────────────────
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	var monthlyIncome float64
	h.db.Raw("SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE user_id = ? AND type = 'income' AND date >= ? AND date < ? AND deleted_at IS NULL",
		userID, startOfMonth, endOfMonth).
		Scan(&monthlyIncome)

	var monthlyExpenses float64
	h.db.Raw("SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE user_id = ? AND type = 'expense' AND date >= ? AND date < ? AND deleted_at IS NULL",
		userID, startOfMonth, endOfMonth).
		Scan(&monthlyExpenses)

	// ── 4. Expense Breakdown by Category ────────────────────────────────────
	var breakdown []CategoryBreakdown
	h.db.Model(&models.Transaction{}).
		Select("COALESCE(NULLIF(category, ''), 'Uncategorized') as category, COALESCE(SUM(amount), 0) as amount").
		Where("user_id = ? AND type = 'expense' AND date >= ? AND date < ?",
			userID, startOfMonth, endOfMonth).
		Group("category").
		Order("amount DESC").
		Scan(&breakdown)
	if breakdown == nil {
		breakdown = []CategoryBreakdown{}
	}

	// ── 5. Last 5 Transactions with account name ─────────────────────────────
	type txRow struct {
		ID          uint
		Date        time.Time
		AccountName string
		Category    string
		Description string
		Amount      float64
		Type        string
	}
	var rawTx []txRow
	h.db.Table("transactions").
		Select("transactions.id, transactions.date, accounts.name as account_name, "+
			"transactions.category, transactions.description, transactions.amount, transactions.type").
		Joins("JOIN accounts ON accounts.id = transactions.account_id AND accounts.deleted_at IS NULL").
		Where("transactions.user_id = ? AND transactions.deleted_at IS NULL", userID).
		Order("transactions.date DESC").
		Limit(5).
		Scan(&rawTx)

	recent := make([]RecentTransaction, len(rawTx))
	for i, r := range rawTx {
		recent[i] = RecentTransaction{
			ID:          r.ID,
			Date:        r.Date,
			AccountName: r.AccountName,
			Category:    r.Category,
			Description: r.Description,
			Amount:      r.Amount,
			Type:        r.Type,
		}
	}

	// ── 6. Weekly Spending (last 7 days, expenses only) ──────────────────────
	sevenDaysAgo := now.AddDate(0, 0, -6)
	startOfWindow := time.Date(sevenDaysAgo.Year(), sevenDaysAgo.Month(), sevenDaysAgo.Day(), 0, 0, 0, 0, now.Location())

	type dayRow struct {
		Day    string
		Amount float64
	}
	var dayRows []dayRow
	h.db.Model(&models.Transaction{}).
		Select("DATE(date) as day, COALESCE(SUM(amount), 0) as amount").
		Where("user_id = ? AND type = 'expense' AND date >= ?", userID, startOfWindow).
		Group("DATE(date)").
		Order("day ASC").
		Scan(&dayRows)

	dayMap := map[string]float64{}
	for _, d := range dayRows {
		dayMap[d.Day] = d.Amount
	}

	weekly := make([]DailySpending, 7)
	for i := 0; i < 7; i++ {
		d := startOfWindow.AddDate(0, 0, i)
		key := d.Format("2006-01-02")
		label := d.Format("Mon")
		weekly[i] = DailySpending{Day: label, Amount: dayMap[key]}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_balance":       totalBalance,
		"monthly_income":      monthlyIncome,
		"monthly_expenses":    monthlyExpenses,
		"expense_breakdown":   breakdown,
		"recent_transactions": recent,
		"weekly_spending":     weekly,
	})
}
