package dto

type CreateCompanyRequest struct {
	// Required
	Name      string `json:"name" binding:"required"`
	TaxNumber string `json:"tax_number" binding:"required"`

	// Optional
	Phone     *string  `json:"phone,omitempty"`
	Address   *string  `json:"address,omitempty"`
	Latitude  *float64 `json:"lat,omitempty"`
	Longitude *float64 `json:"lon,omitempty"`
}

type UpdateCompanyRequest struct {
	// Optional
	Name      *string `json:"name,omitempty"`
	TaxNumber *string `json:"tax_number,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	Address   *string `json:"address,omitempty"`
}

type UpdateCompanyLocationRequest struct {
	// Required
	Latitude  float64 `json:"lat" binding:"required"`
	Longitude float64 `json:"lon" binding:"required"`
}
