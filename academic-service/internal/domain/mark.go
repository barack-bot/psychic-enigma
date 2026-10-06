package domain

type Mark struct {
	ID              int64   `json:"id"`
	AssessmentID    int64   `json:"assessment_id"`
	StudentID       int64   `json:"student_id"`
	Score           float64 `json:"score"`
	Status          string  `json:"status"`
	SubmittedBy     *int64  `json:"submitted_by,omitempty"`
	ApprovedBy      *int64  `json:"approved_by,omitempty"`
	RejectionReason string  `json:"rejection_reason,omitempty"`
}
