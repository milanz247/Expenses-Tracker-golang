package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Name       string `gorm:"size:255;not null" json:"name"`
	Email      string `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password   string `gorm:"size:255;not null" json:"-"`
	Currency   string `gorm:"size:10;default:'LKR'" json:"currency"`
	Language   string `gorm:"size:50;default:'English'" json:"language"`
	ProfilePic string `gorm:"size:500" json:"profile_pic"`
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type UpdateProfileRequest struct {
	Name       string `json:"name" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
	ProfilePic string `json:"profile_pic"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=6"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type UpdatePreferencesRequest struct {
	Currency string `json:"currency" binding:"required"`
	Language string `json:"language" binding:"required"`
}
