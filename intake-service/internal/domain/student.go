package domain

type Student struct {
	ID              int64  `json:"student_id"`
	Name            string `json:"name"`
	IDNumber        string `json:"id_number"`
	Email           string `json:"email,omitempty"`
	DateOfBirth     string `json:"date_of_birth"`
	PhoneNumber     string `json:"phone_number,omitempty"`
	AdmissionNumber string `json:"admission_number,omitempty"`
	Status          string `json:"status"`
}
