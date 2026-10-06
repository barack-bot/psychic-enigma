package domain

type Enrollment struct {
	ID              int64  `json:"id"`
	StudentID       int64  `json:"student_id"`
	AcademicTermID  int64  `json:"academic_term_id"`
	ClassOfferingID *int64 `json:"class_offering_id,omitempty"`
	Status          string `json:"status"`
	ReportedAt      string `json:"reported_at,omitempty"`
	DeferredReason  string `json:"deferred_reason,omitempty"`
}
