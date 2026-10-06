package domain

type Teacher struct {
	ID           int64  `json:"id"`
	EmployeeNo   string `json:"employee_no,omitempty"`
	Name         string `json:"name"`
	Email        string `json:"email,omitempty"`
	DepartmentID *int64 `json:"department_id,omitempty"`
	Status       string `json:"status"`
}
