package models

import (
	"time"

	"gorm.io/gorm"
)

type Budget struct {
	ID         uint           `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt  time.Time      `json:"-"`
	UpdatedAt  time.Time      `json:"-"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	UserID     uint           `gorm:"not null;index" json:"user_id"`
	CategoryID uint           `gorm:"not null" json:"category_id"`
	Amount     float64        `gorm:"not null" json:"amount"`
	Month      int            `gorm:"not null" json:"month"`
	Year       int            `gorm:"not null" json:"year"`
}

type UpsertBudgetRequest struct {
	CategoryID uint    `json:"category_id" binding:"required"`
	Amount     float64 `json:"amount" binding:"required,gt=0"`
	Month      int     `json:"month" binding:"required,min=1,max=12"`
	Year       int     `json:"year" binding:"required"`
}
