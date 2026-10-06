package domain

type ClassOffering struct {
	ID             int64  `json:"id"`
	CourseID       int64  `json:"course_id"`
	AcademicTermID int64  `json:"academic_term_id"`
	Name           string `json:"name"`
	ClassTeacherID *int64 `json:"class_teacher_id,omitempty"`
	Capacity       *int   `json:"capacity,omitempty"`
}

type ClassSubject struct {
	ID              int64  `json:"id"`
	ClassOfferingID int64  `json:"class_offering_id"`
	SubjectID       int64  `json:"subject_id"`
	TeacherID       *int64 `json:"teacher_id,omitempty"`
}
