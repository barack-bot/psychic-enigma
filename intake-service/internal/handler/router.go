package handler

import "net/http"

// Router wires all intake-service handlers onto a mux.
type Router struct {
	students *StudentHandler
	intake   *IntakeHandler
	auth     *AuthHandler
	export   *ExportHandler
	refs     *ReferenceHandler
}

func NewRouter(students *StudentHandler, intake *IntakeHandler, auth *AuthHandler, export *ExportHandler, refs *ReferenceHandler) *Router {
	return &Router{students: students, intake: intake, auth: auth, export: export, refs: refs}
}

func (r *Router) RegisterPublic(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/login", r.auth.Login)
	mux.HandleFunc("/api/applications", r.intake.CreateApplication)
}

func (r *Router) RegisterAdmin(mux *http.ServeMux) {
	mux.HandleFunc("/api/students", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPost:
			r.students.Create(w, req)
		case http.MethodGet:
			r.students.List(w, req)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/students/{id}", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPut:
			r.students.Update(w, req)
		case http.MethodDelete:
			r.students.Delete(w, req)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	r.export.Register(mux)
	r.refs.Register(mux)
}