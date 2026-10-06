package handler

import (
	"bufio"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type StudentHandler struct{ db *sql.DB }
type StudentInput struct {
	Name              string `json:"name"`
	IDNumber          string `json:"id_number"`
	Email             string `json:"email"`
	DateOfBirth       string `json:"date_of_birth"`
	Phone             string `json:"phone_number"`
	KCSEIndexNumber   string `json:"kcse_index_number"`
	KCSEYear          *int   `json:"kcse_year"`
	KCPEIndexNumber   string `json:"kcpe_index_number"`
	KCPEYear          *int   `json:"kcpe_year"`
	ParentName        string `json:"parent_name"`
	ParentPhoneNumber string `json:"parent_phone_number"`
	County            string `json:"county"`
	Ward              string `json:"ward"`
	Location          string `json:"location"`
	Village           string `json:"village"`
	AdmissionNo       string `json:"admission_number"`
	Status            string `json:"status"`
	InitialPassword   string `json:"initial_password"`
}
type StudentUpdateInput struct {
	Name              *string `json:"name"`
	IDNumber          *string `json:"id_number"`
	Email             *string `json:"email"`
	DateOfBirth       *string `json:"date_of_birth"`
	Phone             *string `json:"phone_number"`
	KCSEIndexNumber   *string `json:"kcse_index_number"`
	KCSEYear          *int    `json:"kcse_year"`
	KCPEIndexNumber   *string `json:"kcpe_index_number"`
	KCPEYear          *int    `json:"kcpe_year"`
	ParentName        *string `json:"parent_name"`
	ParentPhoneNumber *string `json:"parent_phone_number"`
	County            *string `json:"county"`
	Ward              *string `json:"ward"`
	Location          *string `json:"location"`
	Village           *string `json:"village"`
	AdmissionNo       *string `json:"admission_number"`
	Status            *string `json:"status"`
	InitialPassword   *string `json:"initial_password"`
	ExpectedVersion   *int64  `json:"expected_version"`
}
type Student struct {
	ID          int64  `json:"student_id"`
	Name        string `json:"name"`
	IDNumber    string `json:"id_number,omitempty"`
	Email       string `json:"email,omitempty"`
	DateOfBirth string `json:"date_of_birth,omitempty"`
	Phone       string `json:"phone_number,omitempty"`
	AdmissionNo string `json:"admission_number,omitempty"`
	Status      string `json:"status"`
	Version     int64  `json:"version"`
	Deleted     bool   `json:"deleted,omitempty"`
}

func NewStudentHandler(db *sql.DB) *StudentHandler { return &StudentHandler{db: db} }

func (h *StudentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input StudentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if input.IDNumber == "" || input.Name == "" || input.DateOfBirth == "" {
		http.Error(w, "name, id_number, and date_of_birth are required", http.StatusBadRequest)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer tx.Rollback()
	var id int64
	var version int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO students (name, id_number, email, date_of_birth, phone_number, kcse_index_number, kcse_year, kcpe_index_number, kcpe_year, parent_name, parent_phone_number, county, ward, location, village, admission_number, status) VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,''),NULLIF($6,''),$7,NULLIF($8,''),$9,NULLIF($10,''),NULLIF($11,''),NULLIF($12,''),NULLIF($13,''),NULLIF($14,''),NULLIF($15,''),NULLIF($16,''),$17) RETURNING student_id, version`, input.Name, input.IDNumber, input.Email, input.DateOfBirth, input.Phone, input.KCSEIndexNumber, input.KCSEYear, input.KCPEIndexNumber, input.KCPEYear, input.ParentName, input.ParentPhoneNumber, input.County, input.Ward, input.Location, input.Village, input.AdmissionNo, input.Status).Scan(&id, &version)
	if err != nil {
		http.Error(w, "student could not be created", http.StatusConflict)
		return
	}
	if input.AdmissionNo == "" {
		input.AdmissionNo = fmt.Sprintf("ADM-%06d", id)
		if _, err := tx.ExecContext(r.Context(), `UPDATE students SET admission_number=$1 WHERE student_id=$2`, input.AdmissionNo, id); err != nil {
			http.Error(w, "admission number could not be generated", 500)
			return
		}
	}
	password := strings.TrimSpace(input.InitialPassword)
	if password == "" {
		password = generateTempPassword()
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "password could not be secured", http.StatusInternalServerError)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE students SET password_hash=$1, must_change_password=true, is_continuing=true WHERE student_id=$2`, string(hash), id); err != nil {
		http.Error(w, "student password could not be created", http.StatusInternalServerError)
		return
	}
	student := Student{ID: id, Name: input.Name, IDNumber: input.IDNumber, Email: input.Email, DateOfBirth: input.DateOfBirth, Phone: input.Phone, AdmissionNo: input.AdmissionNo, Status: input.Status, Version: version}
	if err := h.enqueueEvent(r.Context(), tx, "StudentCreated", student); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	studentMap := map[string]any{"student_id": student.ID, "name": student.Name, "id_number": student.IDNumber, "email": student.Email, "date_of_birth": student.DateOfBirth, "phone_number": student.Phone, "admission_number": student.AdmissionNo, "status": student.Status, "version": student.Version, "initial_password": password}
	writeJSON(w, http.StatusCreated, studentMap)
}

func generateTempPassword() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	bytes := make([]byte, 10)
	if _, err := rand.Read(bytes); err != nil {
		return "TempPass!1"
	}
	for i := range bytes {
		bytes[i] = alphabet[int(bytes[i])%len(alphabet)]
	}
	return string(bytes)
}

func (h *StudentHandler) ImportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "CSV file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	reader := csv.NewReader(bufio.NewReader(file))
	records, err := reader.ReadAll()
	if err != nil {
		http.Error(w, "CSV could not be parsed", http.StatusBadRequest)
		return
	}
	if len(records) < 2 {
		writeJSON(w, http.StatusOK, []map[string]any{})
		return
	}
	headers := make(map[string]int)
	for idx, header := range records[0] {
		headers[strings.TrimSpace(strings.ToLower(header))] = idx
	}
	for _, expected := range []string{"name", "id_number", "date_of_birth", "email", "phone_number"} {
		if _, ok := headers[expected]; !ok {
			http.Error(w, fmt.Sprintf("CSV header %q is required", expected), http.StatusBadRequest)
			return
		}
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	results := make([]map[string]any, 0, len(records)-1)
	for rowIndex, row := range records[1:] {
		fileRow := rowIndex + 2
		if len(row) == 0 || strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		if len(row) <= maxIndex(headers) {
			results = append(results, map[string]any{"row": fileRow, "error": "row is incomplete"})
			continue
		}
		name := strings.TrimSpace(row[headers["name"]])
		idNumber := strings.TrimSpace(row[headers["id_number"]])
		birth := strings.TrimSpace(row[headers["date_of_birth"]])
		email := strings.TrimSpace(row[headers["email"]])
		phone := strings.TrimSpace(row[headers["phone_number"]])
		if name == "" || idNumber == "" || birth == "" {
			results = append(results, map[string]any{"row": fileRow, "error": "name, id_number, and date_of_birth are required"})
			continue
		}
		password := generateTempPassword()
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			results = append(results, map[string]any{"row": fileRow, "error": "password could not be secured: " + err.Error()})
			continue
		}
		var studentID int64
		var admissionNo sql.NullString
		if err := tx.QueryRowContext(r.Context(), `INSERT INTO students (name, id_number, email, date_of_birth, phone_number, admission_number, password_hash, must_change_password, is_continuing, status) VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,''),NULLIF($6,''),$7,true,true,'ACTIVE') RETURNING student_id, admission_number`, name, idNumber, email, birth, phone, "", string(hash)).Scan(&studentID, &admissionNo); err != nil {
			results = append(results, map[string]any{"row": fileRow, "error": "student import failed: " + err.Error()})
			continue
		}
		admission := admissionNo.String
		if !admissionNo.Valid || admission == "" {
			admission = fmt.Sprintf("ADM-%06d", studentID)
			if _, err := tx.ExecContext(r.Context(), `UPDATE students SET admission_number=$1 WHERE student_id=$2`, admission, studentID); err != nil {
				results = append(results, map[string]any{"row": fileRow, "error": "admission number could not be generated: " + err.Error()})
				continue
			}
		}
		student := Student{ID: studentID, Name: name, IDNumber: idNumber, Email: email, DateOfBirth: birth, Phone: phone, AdmissionNo: admission, Status: "ACTIVE", Version: 1}
		if err := h.enqueueEvent(r.Context(), tx, "StudentCreated", student); err != nil {
			results = append(results, map[string]any{"row": fileRow, "error": "student event could not be queued: " + err.Error()})
			continue
		}
		results = append(results, map[string]any{"row": fileRow, "student_id": studentID, "admission_number": admission, "name": name, "initial_password": password})
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, results)
}

func maxIndex(headers map[string]int) int {
	max := -1
	for _, idx := range headers {
		if idx > max {
			max = idx
		}
	}
	return max
}

func (h *StudentHandler) List(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	query := `SELECT student_id, name, id_number, COALESCE(email,''), date_of_birth, COALESCE(phone_number,''), COALESCE(admission_number,''), status, version FROM students WHERE deleted_at IS NULL`
	args := []any{}
	if r.URL.Query().Get("include_deleted") == "true" {
		query = `SELECT student_id, name, id_number, COALESCE(email,''), date_of_birth, COALESCE(phone_number,''), COALESCE(admission_number,''), status, version FROM students`
	}
	if search != "" {
		query += ` AND (LOWER(name) LIKE $1 OR LOWER(COALESCE(admission_number,'')) LIKE $1)`
		args = append(args, "%"+strings.ToLower(search)+"%")
	}
	query += ` ORDER BY student_id`
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 50
	}
	if size > 200 {
		size = 200
	}
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", size, (page-1)*size)
	w.Header().Set("X-Page", strconv.Itoa(page))
	w.Header().Set("X-Page-Size", strconv.Itoa(size))
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer rows.Close()
	students := make([]Student, 0)
	for rows.Next() {
		var student Student
		if err := rows.Scan(&student.ID, &student.Name, &student.IDNumber, &student.Email, &student.DateOfBirth, &student.Phone, &student.AdmissionNo, &student.Status, &student.Version); err != nil {
			http.Error(w, "database error", 500)
			return
		}
		students = append(students, student)
	}
	writeJSON(w, http.StatusOK, students)
}

func (h *StudentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid student id", 400)
		return
	}
	var input StudentUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if input.ExpectedVersion == nil || *input.ExpectedVersion < 1 {
		http.Error(w, "expected_version is required", http.StatusPreconditionRequired)
		return
	}
	nameValue := ""
	if input.Name != nil {
		nameValue = *input.Name
	}
	idNumberValue := ""
	if input.IDNumber != nil {
		idNumberValue = *input.IDNumber
	}
	emailValue := ""
	if input.Email != nil {
		emailValue = *input.Email
	}
	dateOfBirthValue := ""
	if input.DateOfBirth != nil {
		dateOfBirthValue = *input.DateOfBirth
	}
	phoneValue := ""
	if input.Phone != nil {
		phoneValue = *input.Phone
	}
	kcseIndexValue := ""
	if input.KCSEIndexNumber != nil {
		kcseIndexValue = *input.KCSEIndexNumber
	}
	kcseYearValue := -1
	if input.KCSEYear != nil {
		kcseYearValue = *input.KCSEYear
	}
	kcpeIndexValue := ""
	if input.KCPEIndexNumber != nil {
		kcpeIndexValue = *input.KCPEIndexNumber
	}
	kcpeYearValue := -1
	if input.KCPEYear != nil {
		kcpeYearValue = *input.KCPEYear
	}
	parentNameValue := ""
	if input.ParentName != nil {
		parentNameValue = *input.ParentName
	}
	parentPhoneValue := ""
	if input.ParentPhoneNumber != nil {
		parentPhoneValue = *input.ParentPhoneNumber
	}
	countyValue := ""
	if input.County != nil {
		countyValue = *input.County
	}
	wardValue := ""
	if input.Ward != nil {
		wardValue = *input.Ward
	}
	locationValue := ""
	if input.Location != nil {
		locationValue = *input.Location
	}
	villageValue := ""
	if input.Village != nil {
		villageValue = *input.Village
	}
	admissionValue := ""
	if input.AdmissionNo != nil {
		admissionValue = *input.AdmissionNo
	}
	statusValue := ""
	if input.Status != nil {
		statusValue = *input.Status
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer tx.Rollback()
	var student Student
	err = tx.QueryRowContext(r.Context(), `UPDATE students SET name=COALESCE(NULLIF($1,''), name), id_number=COALESCE(NULLIF($2,''), id_number), email=COALESCE(NULLIF($3,''), email), date_of_birth=COALESCE(NULLIF($4,'')::date, date_of_birth), phone_number=COALESCE(NULLIF($5,''), phone_number), kcse_index_number=COALESCE(NULLIF($6,''), kcse_index_number), kcse_year=CASE WHEN $7 = -1 THEN kcse_year ELSE $7 END, kcpe_index_number=COALESCE(NULLIF($8,''), kcpe_index_number), kcpe_year=CASE WHEN $9 = -1 THEN kcpe_year ELSE $9 END, parent_name=COALESCE(NULLIF($10,''), parent_name), parent_phone_number=COALESCE(NULLIF($11,''), parent_phone_number), county=COALESCE(NULLIF($12,''), county), ward=COALESCE(NULLIF($13,''), ward), location=COALESCE(NULLIF($14,''), location), village=COALESCE(NULLIF($15,''), village), admission_number=COALESCE(NULLIF($16,''), admission_number), status=COALESCE(NULLIF($17,''), status), version=version+1 WHERE student_id=$18 AND version=$19 AND deleted_at IS NULL RETURNING student_id, name, id_number, COALESCE(email,''), date_of_birth, COALESCE(phone_number,''), COALESCE(admission_number,''), status, version`, nameValue, idNumberValue, emailValue, dateOfBirthValue, phoneValue, kcseIndexValue, kcseYearValue, kcpeIndexValue, kcpeYearValue, parentNameValue, parentPhoneValue, countyValue, wardValue, locationValue, villageValue, admissionValue, statusValue, id, *input.ExpectedVersion).Scan(&student.ID, &student.Name, &student.IDNumber, &student.Email, &student.DateOfBirth, &student.Phone, &student.AdmissionNo, &student.Status, &student.Version)
	if err == sql.ErrNoRows {
		http.Error(w, "student not found or version conflict", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "student could not be updated", http.StatusUnprocessableEntity)
		return
	}
	if input.InitialPassword != nil && strings.TrimSpace(*input.InitialPassword) != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(*input.InitialPassword)), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "password could not be secured", http.StatusInternalServerError)
			return
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE students SET password_hash=$1, must_change_password=true WHERE student_id=$2`, string(hash), id); err != nil {
			http.Error(w, "student password could not be updated", http.StatusInternalServerError)
			return
		}
	}
	if err := h.enqueueEvent(r.Context(), tx, "StudentUpdated", student); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	writeJSON(w, http.StatusOK, student)
}

func (h *StudentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid student id", http.StatusBadRequest)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer tx.Rollback()
	var student Student
	err = tx.QueryRowContext(r.Context(), `UPDATE students SET deleted_at = CURRENT_TIMESTAMP, status = 'WITHDRAWN' WHERE student_id = $1 AND deleted_at IS NULL RETURNING student_id, name, id_number, COALESCE(email,''), date_of_birth, COALESCE(phone_number,''), COALESCE(admission_number,''), status, version`, id).Scan(&student.ID, &student.Name, &student.IDNumber, &student.Email, &student.DateOfBirth, &student.Phone, &student.AdmissionNo, &student.Status, &student.Version)
	if err != nil {
		http.Error(w, "student not found", http.StatusNotFound)
		return
	}
	student.Deleted = true
	if err := h.enqueueEvent(r.Context(), tx, "StudentDeleted", student); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM auth_sessions WHERE student_id = $1`, id); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *StudentHandler) enqueueEvent(ctx context.Context, tx *sql.Tx, eventType string, student Student) error {
	eventIDBytes := make([]byte, 16)
	if _, err := rand.Read(eventIDBytes); err != nil {
		return err
	}
	payload, err := json.Marshal(student)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO student_events_outbox (event_id, event_type, student_id, version, payload) VALUES ($1, $2, $3, $4, $5)`, fmt.Sprintf("student-%x", eventIDBytes), eventType, student.ID, student.Version, payload)
	return err
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
