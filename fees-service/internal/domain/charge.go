package domain

type Charge struct {
	ID             int64   `json:"id"`
	StudentID      int64   `json:"student_id"`
	AcademicTermID int64   `json:"academic_term_id"`
	Amount         float64 `json:"amount"`
	Description    string  `json:"description,omitempty"`
}
