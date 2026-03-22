package routes

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"expense-tracker-api/handlers"
	"expense-tracker-api/middleware"
)

func SetupRoutes(authHandler *handlers.AuthHandler, accountHandler *handlers.AccountHandler, categoryHandler *handlers.CategoryHandler, summaryHandler *handlers.SummaryHandler, jwtSecret string) *gin.Engine {
	r := gin.Default()

	// CORS must be registered before any routes
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
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

		// Accounts
		protected.POST("/accounts", accountHandler.CreateAccount)
		protected.GET("/accounts", accountHandler.GetAccounts)

		// Transactions
		protected.POST("/transactions", accountHandler.CreateTransaction)
		protected.GET("/transactions", accountHandler.GetTransactions)

		// Categories
		protected.POST("/categories", categoryHandler.CreateCategory)
		protected.GET("/categories", categoryHandler.GetCategories)
		protected.DELETE("/categories/:id", categoryHandler.DeleteCategory)

		// Summary
		protected.GET("/summary", summaryHandler.GetSummary)
	}

	return r
}
