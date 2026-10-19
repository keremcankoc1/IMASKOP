package model

import (
	"time"

	"github.com/google/uuid"
)

type VehicleType string

const (
	TypeTruck     VehicleType = "TRUCK"
	TypeExcavator VehicleType = "EXCAVATOR"
	TypeBackhoe   VehicleType = "BACKHOE"
)

type VehicleStatus string

const (
	StatusAvailable   VehicleStatus = "AVAILABLE"
	StatusReserved    VehicleStatus = "RESERVED"
	StatusInUse       VehicleStatus = "IN_USE"
	StatusMaintenance VehicleStatus = "MAINTENANCE"
)

type Vehicle struct {
	ID        uuid.UUID     `json:"id"`
	CompanyID uuid.UUID     `json:"company_id"`
	Type      VehicleType   `json:"type"`
	Status    VehicleStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Plate     string        `json:"plate"`

	// Optional
	Brand             *string    `json:"brand,omitempty"`
	Model             *string    `json:"model,omitempty"`
	Year              *int       `json:"year,omitempty"`
	Passengers        *int       `json:"passengers,omitempty"`
	CargoKg           *float64   `json:"cargo_kg,omitempty"`
	Latitude          *float64   `json:"lat,omitempty"`
	Longitude         *float64   `json:"lon,omitempty"`
	PricePerKm        *float64   `json:"price_per_km,omitempty"`
	LocationUpdatedAt *time.Time `json:"location_updated_at,omitempty"`
}
