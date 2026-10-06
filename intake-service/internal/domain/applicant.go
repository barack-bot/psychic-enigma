package domain

type Applicant struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
	CourseID    *int64 `json:"course_id,omitempty"`
	Status      string `json:"status"`
}
