package domain

type Student struct {
	ID          int64  `json:"student_id"`
	Name        string `json:"name"`
	AdmissionNo string `json:"admission_number,omitempty"`
	Status      string `json:"status"`
}
