package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// ReferenceHandler supplies lookup lists for forms: courses, levels, departments.
type ReferenceHandler struct{ db *sql.DB }

func NewReferenceHandler(db *sql.DB) *ReferenceHandler { return &ReferenceHandler{db: db} }

func (h *ReferenceHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/reference/courses", h.courses)
	mux.HandleFunc("/api/reference/departments", h.departments)
}

func (h *ReferenceHandler) courses(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,name,level_id,department_id FROM courses ORDER BY name`); if err != nil { http.Error(w, "database error", 500); return }; defer rows.Close(); var result []map[string]any; for rows.Next() { var id, level int64; var name string; var department *int64; if rows.Scan(&id, &name, &level, &department) == nil { result = append(result, map[string]any{"id": id, "name": name, "level_id": level, "department_id": department}) } }; _ = json.NewEncoder(w).Encode(result)
}

func (h *ReferenceHandler) departments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,name FROM departments ORDER BY name`); if err != nil { http.Error(w, "database error", 500); return }; defer rows.Close(); var result []map[string]any; for rows.Next() { var id int64; var name string; if rows.Scan(&id, &name) == nil { result = append(result, map[string]any{"id": id, "name": name}) } }; _ = json.NewEncoder(w).Encode(result)
}