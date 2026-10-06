package domain

type Payment struct {
	ID                   int64   `json:"id"`
	StudentID            int64   `json:"student_id"`
	AcademicTermID       *int64  `json:"academic_term_id,omitempty"`
	Source               string  `json:"source"`
	Method               string  `json:"method"`
	Amount               float64 `json:"amount"`
	TransactionReference string  `json:"transaction_reference"`
	Status               string  `json:"status"`
}
