package model

import (
	"time"

	"github.com/google/uuid"
)

type Company struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	TaxNumber string    `json:"tax_number"`

	// Optional
	Phone   *string `json:"phone,omitempty"`
	Address *string `json:"address,omitempty"`

	// Location (Optional)
	Latitude  *float64 `json:"lat,omitempty"`
	Longitude *float64 `json:"lon,omitempty"`

	OwnerID   uuid.UUID `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
