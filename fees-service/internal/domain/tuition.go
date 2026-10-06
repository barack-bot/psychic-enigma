package domain

type TuitionRate struct {
	ID             int64   `json:"id"`
	AcademicTermID int64   `json:"academic_term_id"`
	Amount         float64 `json:"amount"`
}
