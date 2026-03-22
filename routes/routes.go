package routes

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"expense-tracker-api/handlers"
	"expense-tracker-api/middleware"
)

func SetupRoutes(authHandler *handlers.AuthHandler, accountHandler *handlers.AccountHandler, categoryHandler *handlers.CategoryHandler, budgetHandler *handlers.BudgetHandler, debtHandler *handlers.DebtHandler, summaryHandler *handlers.SummaryHandler, jwtSecret string) *gin.Engine {
	r := gin.Default()

	// CORS must be registered before any routes
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://ledgr.milanmadusanka.me"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Health check
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Public auth routes
	r.POST("/register", authHandler.Register)
	r.POST("/login", authHandler.Login)

	// Protected routes
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware(jwtSecret))
	{
		protected.GET("/me", authHandler.GetMe)

		// User profile & preferences
		protected.PUT("/user/profile", authHandler.UpdateProfile)
		protected.PUT("/user/password", authHandler.ChangePassword)
		protected.PUT("/user/preferences", authHandler.UpdatePreferences)

		// Accounts
		protected.POST("/accounts", accountHandler.CreateAccount)
		protected.GET("/accounts", accountHandler.GetAccounts)

		// Transactions
		protected.POST("/transactions", accountHandler.CreateTransaction)
		protected.GET("/transactions", accountHandler.GetTransactions)
		protected.DELETE("/transactions/:id", accountHandler.DeleteTransaction)

		// Categories
		protected.POST("/categories", categoryHandler.CreateCategory)
		protected.GET("/categories", categoryHandler.GetCategories)
		protected.DELETE("/categories/:id", categoryHandler.DeleteCategory)

		// Budgets
		protected.POST("/budgets", budgetHandler.UpsertBudget)
		protected.GET("/budgets", budgetHandler.GetBudgets)
		protected.DELETE("/budgets/:id", budgetHandler.DeleteBudget)

		// Debts
		protected.POST("/debts", debtHandler.CreateDebt)
		protected.GET("/debts", debtHandler.GetDebts)
		protected.GET("/debts/summary", debtHandler.GetDebtSummary)
		protected.POST("/debts/:id/repay", debtHandler.RepayDebt)
		protected.DELETE("/debts/:id", debtHandler.DeleteDebt)

		// Summary
		protected.GET("/summary", summaryHandler.GetSummary)
	}

	return r
}
