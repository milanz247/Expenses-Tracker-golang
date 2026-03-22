package models

import (
	"time"

	"gorm.io/gorm"
)

type Debt struct {
	ID            uint           `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"-"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	UserID        uint           `gorm:"not null;index" json:"user_id"`
	AccountID     uint           `gorm:"not null" json:"account_id"`
	PersonName    string         `gorm:"size:255;not null" json:"person_name"`
	Description   string         `gorm:"size:1000" json:"description"`
	Amount        float64        `gorm:"not null" json:"amount"`
	PaidAmount    float64        `gorm:"default:0" json:"paid_amount"`
	Type          string         `gorm:"size:10;not null" json:"type"` // LEND or BORROW
	DueDate       *time.Time     `json:"due_date,omitempty"`
	Status        string         `gorm:"size:10;not null;default:OPEN" json:"status"` // OPEN or CLOSED
	TransactionID *uint          `gorm:"index" json:"transaction_id,omitempty"`       // auto-created initial transaction
}

type CreateDebtRequest struct {
	AccountID   uint       `json:"account_id" binding:"required"`
	PersonName  string     `json:"person_name" binding:"required"`
	Description string     `json:"description"`
	Amount      float64    `json:"amount" binding:"required,gt=0"`
	Type        string     `json:"type" binding:"required,oneof=LEND BORROW"`
	DueDate     *time.Time `json:"due_date"`
}

type RepayDebtRequest struct {
	AccountID uint    `json:"account_id" binding:"required"`
	Amount    float64 `json:"amount" binding:"required,gt=0"`
}
