package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"school-platform/academic-service/internal/config"
	"school-platform/academic-service/internal/handler"
	"school-platform/academic-service/internal/middleware"
	"school-platform/academic-service/internal/repository/postgres"
	"school-platform/academic-service/internal/service"
	"school-platform/shared/httpkit"
)

func main() {
	_ = config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	dsn := getenv("ACADEMIC_DATABASE_URL", getenv("DATABASE_URL", ""))
	if dsn == "" {
		log.Fatal("ACADEMIC_DATABASE_URL or DATABASE_URL is required")
	}
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	go func() { ticker := time.NewTicker(time.Hour); defer ticker.Stop(); for { select { case <-ctx.Done(): return; case <-ticker.C: _, _ = db.Exec(ctx, `DELETE FROM auth_sessions WHERE expires_at <= CURRENT_TIMESTAMP`) } } }()
	publicMux := http.NewServeMux()
	intakeTarget, err := url.Parse(getenv("INTAKE_URL", "http://intake-service:8080"))
	if err != nil { log.Fatal(err) }
	intakeStudentsProxy := httputil.NewSingleHostReverseProxy(intakeTarget)
	publicMux.Handle("/api/auth/login", httputil.NewSingleHostReverseProxy(intakeTarget))
	publicMux.Handle("/api/intake-students", middleware.RequireStaff(db, httpkit.LimitBody(1<<20, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/api/students"
		intakeStudentsProxy.ServeHTTP(w, r)
	}))))
	publicMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	staffMux := http.NewServeMux()
	publicMux.Handle("/staff/", http.StripPrefix("/staff/", http.FileServer(http.Dir("web"))))
	academicHandler := handler.NewAcademicHandler(db)
	academicHandler.Register(staffMux)
	markRepository := postgres.NewMarkRepository(db)
	classRepository := postgres.NewClassRepository(db)
	teacherHandler := handler.NewTeacherHandler(
		db,
		service.NewMarkSubmissionService(markRepository),
		service.NewRosterService(classRepository),
	)
	teacherHandler.Register(staffMux)
	hodHandler := handler.NewHODHandler(db, service.NewMarkApprovalService(markRepository))
	hodHandler.Register(staffMux)
	enrollmentHandler := handler.NewEnrollmentHandler(service.NewEnrollmentService(postgres.NewEnrollmentRepository(db)))
	enrollmentHandler.Register(staffMux)
	adminHandler := handler.NewAdminHandler(db, service.NewTeacherAssignmentService(classRepository))
	adminHandler.Register(staffMux)

	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/intake-students" || r.URL.Path == "/staff" || r.URL.Path == "/staff/" || strings.HasPrefix(r.URL.Path, "/staff/") {
			publicMux.ServeHTTP(w, r)
			return
		}
		middleware.RequireStaff(db, staffMux).ServeHTTP(w, r)
	})
	secured := httpkit.SecurityHeaders(httpkit.LimitBody(1<<20, httpkit.RequestID(httpkit.Recover(httpkit.Logging(root)))))
	server := &http.Server{Addr: ":" + getenv("ACADEMIC_PORT", "8082"), Handler: secured, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("academic service: %v", err)
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
