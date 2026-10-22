package model

import (
	"time"

	"github.com/google/uuid"
)

type ReservationStatus string

const (
	ReservationStatusPending   ReservationStatus = "PENDING"
	ReservationStatusConfirmed ReservationStatus = "CONFIRMED"
	ReservationStatusCompleted ReservationStatus = "COMPLETED"
	ReservationStatusCancelled ReservationStatus = "CANCELLED"
)

type Reservation struct {
	ID uuid.UUID `json:"id"`

	// Foreign keys:
	VehicleID  uuid.UUID `json:"vehicle_id"`
	RequestID  uuid.UUID `json:"request_id"`
	CompanyID  uuid.UUID `json:"company_id"`
	CustomerID uuid.UUID `json:"customer_id"`

	// Situation and Price:
	Status     ReservationStatus `json:"status"`
	TotalPrice *float64          `json:"total_price,omitempty"` // optional

	// Lifecycle Timestamps:
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"` // The moment the mission is approved, optional
	CompletedAt *time.Time `json:"completed_at,omitempty"` // The moment the mission is completed, optional
	CancelledAt *time.Time `json:"cancelled_at,omitempty"` // The moment the mission is cancelled, optional

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
}
