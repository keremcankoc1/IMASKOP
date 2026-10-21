package dto

type ConfirmReservationRequest struct {
	// Optional
	TotalPrice *float64 `json:"total_price,omitempty"`
}

type CancelReservationRequest struct {
	// Optional
	Reason *string `json:"reason,omitempty"`
}

type CompleteReservationRequest struct {
	// Optional
	Note *string `json:"note,omitempty"`
}
