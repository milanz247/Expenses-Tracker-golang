package main

import (
	"fmt"
	"log"

	"expense-tracker-api/config"
	"expense-tracker-api/db"
	"expense-tracker-api/handlers"
	"expense-tracker-api/routes"
)

func main() {
	cfg := config.Load()

	database := db.Connect(cfg)

	authHandler := handlers.NewAuthHandler(database, cfg.JWTSecret)
	accountHandler := handlers.NewAccountHandler(database)
	categoryHandler := handlers.NewCategoryHandler(database)
	summaryHandler := handlers.NewSummaryHandler(database)

	router := routes.SetupRoutes(authHandler, accountHandler, categoryHandler, summaryHandler, cfg.JWTSecret)

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("Server starting on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
