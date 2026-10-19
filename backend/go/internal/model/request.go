package model

import (
	"time"

	"github.com/google/uuid"
)

type RequestStatus string

const (
	RequestStatusPending   RequestStatus = "PENDING"
	RequestStatusMatched   RequestStatus = "MATCHED"
	RequestStatusReserved  RequestStatus = "RESERVED"
	RequestStatusCancelled RequestStatus = "CANCELLED"
)

type Request struct {
	ID          uuid.UUID `json:"id"`
	RequesterID uuid.UUID `json:"requester_id"` // The AFAD user who opened the request

	// Location to be acquired (Pickup):
	PickUpLat float64 `json:"pick_up_lat"`
	PickUpLon float64 `json:"pick_up_lon"`

	// Arrival location:
	DropoffLat *float64 `json:"dropoff_lat,omitempty"` // optional
	DropoffLon *float64 `json:"dropoff_lon,omitempty"` // optional

	TypeOfVehicle VehicleType `json:"vehicle_type"`
	Passengers    *int        `json:"passengers,omitempty"` // optional

	// Times
	StartAt time.Time  `json:"start_at"`
	EndAt   *time.Time `json:"end_at"` // optional

	Status RequestStatus `json:"status"`

	// Relationships
	MatchedVehicleID *uuid.UUID `json:"matched_vehicle_id,omitempty"` // optional
	ReservationID    *uuid.UUID `json:"reservation_id,omitempty"`     // optional

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
