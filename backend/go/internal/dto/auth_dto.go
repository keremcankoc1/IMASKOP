package dto

import "imaskop/internal/model"

type RegisterRequest struct {
	Email    string          `json:"email" binding:"required,email"`    // Required
	Password string          `json:"password" binding:"required,min=8"` // Min 8 characters
	Name     string          `json:"name" binding:"required"`
	Role     *model.UserRole `json:"role,omitempty"` // optional
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}
