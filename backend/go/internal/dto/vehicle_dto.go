package dto

import (
	"imaskop/internal/model"

	"github.com/google/uuid"
)

type CreateVehicleRequest struct {
	// Required
	CompanyID uuid.UUID         `json:"company_id" binding:"required"`
	Plate     string            `json:"plate" binding:"required"`
	Type      model.VehicleType `json:"type" binding:"required"`

	// Optional
	Brand      *string  `json:"brand,omitempty"`
	Model      *string  `json:"model,omitempty"`
	Year       *int     `json:"year,omitempty"`
	Passengers *int     `json:"passengers,omitempty"`
	CargoKg    *float64 `json:"cargo_kg,omitempty"`
	Latitude   *float64 `json:"lat,omitempty"`
	Longitude  *float64 `json:"lon,omitempty"`
	PricePerKm *float64 `json:"price_per_km,omitempty"`
}

type UpdateVehicleRequest struct {
	// Optional
	CompanyID  *uuid.UUID           `json:"company_id,omitempty"`
	Plate      *string              `json:"plate,omitempty"`
	Type       *model.VehicleType   `json:"type,omitempty"`
	Brand      *string              `json:"brand,omitempty"`
	Model      *string              `json:"model,omitempty"`
	Year       *int                 `json:"year,omitempty"`
	Passengers *int                 `json:"passengers,omitempty"`
	CargoKg    *float64             `json:"cargo_kg,omitempty"`
	Latitude   *float64             `json:"lat,omitempty"`
	Longitude  *float64             `json:"lon,omitempty"`
	PricePerKm *float64             `json:"price_per_km,omitempty"`
	Status     *model.VehicleStatus `json:"status,omitempty"`
}

type UpdateVehicleLocationRequest struct {
	Latitude  float64 `json:"lat" binding:"required"`
	Longitude float64 `json:"lon" binding:"required"`
}

type SearchVehicleQuery struct {
	Type          *model.VehicleType   `form:"type"`
	Latitude      *float64             `form:"lat"`
	Longitude     *float64             `form:"lon"`
	RadiusKm      *float64             `form:"radius_km"`
	MinPassengers *int                 `form:"min_passengers"`
	Status        *model.VehicleStatus `form:"status"`
}
