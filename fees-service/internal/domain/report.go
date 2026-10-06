package domain

type Balance struct {
	StudentID    int64   `json:"student_id"`
	TotalCharges float64 `json:"total_charges"`
	TotalWaived  float64 `json:"total_waived"`
	TotalPaid    float64 `json:"total_paid"`
	Balance      float64 `json:"balance"`
}
