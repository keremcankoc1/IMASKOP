package model

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleAdmin    UserRole = "ADMIN"
	RoleCompany  UserRole = "COMPANY"
	RoleCustomer UserRole = "CUSTOMER"
)

type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Name         string     `json:"name"`
	Role         UserRole   `json:"role"`
	CompanyID    *uuid.UUID `json:"company_id,omitempty"` // optional
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
