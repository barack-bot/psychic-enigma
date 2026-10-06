package domain

type Subject struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Code         string `json:"code,omitempty"`
	DepartmentID *int64 `json:"department_id,omitempty"`
}
