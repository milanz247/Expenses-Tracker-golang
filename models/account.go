package models

import (
	"time"

	"gorm.io/gorm"
)

type Account struct {
	ID        uint           `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	Name      string         `gorm:"size:255;not null" json:"name"`
	Type      string         `gorm:"size:50;not null" json:"type"`
	Balance   float64        `gorm:"default:0" json:"balance"`
}

type Transaction struct {
	ID          uint           `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt   time.Time      `json:"-"`
	UpdatedAt   time.Time      `json:"-"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	AccountID   uint           `gorm:"not null" json:"account_id"`
	ToAccountID *uint          `json:"to_account_id,omitempty"`
	Amount      float64        `gorm:"not null" json:"amount"`
	Type        string         `gorm:"size:50;not null" json:"type"`
	Category    string         `gorm:"size:255" json:"category"`
	Description string         `gorm:"size:1000" json:"description"`
	Date        time.Time      `json:"date"`
}

type CreateAccountRequest struct {
	Name    string  `json:"name" binding:"required"`
	Type    string  `json:"type" binding:"required,oneof=wallet bank card"`
	Balance float64 `json:"balance"`
}

type CreateTransactionRequest struct {
	AccountID   uint      `json:"account_id" binding:"required"`
	ToAccountID *uint     `json:"to_account_id"`
	Amount      float64   `json:"amount" binding:"required,gt=0"`
	Type        string    `json:"type" binding:"required,oneof=income expense transfer"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
}

type Category struct {
	ID        uint           `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	Name      string         `gorm:"size:255;not null" json:"name"`
	Type      string         `gorm:"size:50;not null" json:"type"`
}

type CreateCategoryRequest struct {
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required,oneof=income expense transfer"`
}
