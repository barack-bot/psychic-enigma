package domain

type LedgerEntry struct {
	ID        int64   `json:"id"`
	StudentID int64   `json:"student_id"`
	Kind      string  `json:"kind"`
	Amount    float64 `json:"amount"`
	Reference string  `json:"reference,omitempty"`
}
