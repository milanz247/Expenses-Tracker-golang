package db

import (
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"expense-tracker-api/config"
	"expense-tracker-api/models"
)

func Connect(cfg *config.Config) *gorm.DB {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("Connected to MySQL database")

	// Auto-migrate tables
	if err := db.AutoMigrate(&models.User{}, &models.Account{}, &models.Transaction{}, &models.Category{}, &models.Budget{}, &models.Debt{}); err != nil {
		log.Fatalf("Failed to auto-migrate: %v", err)
	}

	fmt.Println("Database migration completed")
	return db
}
