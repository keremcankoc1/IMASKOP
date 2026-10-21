package dto

import (
	"imaskop/internal/model"
	"time"

	"github.com/google/uuid"
)

type CreateRequestRequest struct { // AFAD talep oluşturma
	PickupLat float64 `json:"pick_up_lat" binding:"required"`
	PickupLon float64 `json:"pick_up_lon" binding:"required"`

	// Optional
	DropoffLat  *float64          `json:"dropoff_lat,omitempty"`
	DropoffLon  *float64          `json:"dropoff_lon,omitempty"`
	Passengers  *int              `json:"passengers,omitempty"`
	VehicleType model.VehicleType `json:"vehicle_type" binding:"required"`
	EndAt       *time.Time        `json:"end_at,omitempty"`

	StartAt time.Time `json:"start_at" binding:"required"`
}

type MatchRequestQuery struct {
	RadiusKm *float64 `json:"radius_km,omitempty"`
}

type ReserveRequestRequest struct {
	VehicleID uuid.UUID `json:"vehicle_id" binding:"required"`
}

type CancelRequestRequest struct {
	Reason *string `json:"reason,omitempty"`
}
