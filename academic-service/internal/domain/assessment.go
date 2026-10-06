package domain

type Assessment struct {
	ID             int64    `json:"id"`
	ClassSubjectID int64    `json:"class_subject_id"`
	Name           string   `json:"name"`
	AssessmentType string   `json:"assessment_type"`
	MaxScore       float64  `json:"max_score"`
	WeightPercent  *float64 `json:"weight_percent,omitempty"`
	CreatedBy      *int64   `json:"created_by,omitempty"`
}
