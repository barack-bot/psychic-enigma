package main

import (
	"context"
	"database/sql"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"school-platform/intake-service/internal/config"
	intakeevents "school-platform/intake-service/internal/events"
	"school-platform/intake-service/internal/handler"
	intakemiddleware "school-platform/intake-service/internal/middleware"
	sharedevents "school-platform/shared/events"
	"school-platform/shared/httpkit"
	"strings"
	"syscall"
	"time"
)

func main() {
	_ = config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	dsn := getenv("INTAKE_DATABASE_URL", getenv("DATABASE_URL", ""))
	if dsn == "" {
		log.Fatal("INTAKE_DATABASE_URL or DATABASE_URL is required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	var pingErr error
	for attempt := 0; attempt < 30; attempt++ {
		if pingErr = db.PingContext(ctx); pingErr == nil {
			break
		}
		select {
		case <-ctx.Done():
			log.Fatal(ctx.Err())
		case <-time.After(time.Second):
		}
	}
	if pingErr != nil {
		log.Fatal(pingErr)
	}
	go func() { ticker := time.NewTicker(time.Hour); defer ticker.Stop(); for { select { case <-ctx.Done(): return; case <-ticker.C: _, _ = db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE expires_at <= CURRENT_TIMESTAMP`) } } }()
	students := handler.NewStudentHandler(db)
	auth := handler.NewAuthHandler(db, requiredJWTSecret())
	intake := handler.NewIntakeHandler(db)
	exportHandler := handler.NewExportHandler(db)
	referenceHandler := handler.NewReferenceHandler(db)
	registrationLimiter := httpkit.NewRateLimiter(10, time.Minute)
	loginLimiter := httpkit.NewRateLimiter(10, time.Minute)
	applicationLimiter := httpkit.NewRateLimiter(5, time.Minute)
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			http.Error(w, "unavailable", 503)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	publicMux.Handle("/api/auth/login", loginLimiter.Middleware(http.HandlerFunc(auth.Login)))
	publicMux.HandleFunc("/api/auth/logout", auth.Logout)
	publicMux.Handle("/api/applications", applicationLimiter.Middleware(http.HandlerFunc(intake.CreateApplication)))
	publicMux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	publicMux.HandleFunc("/intake-form.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "templates/intake-form.html")
	})
	publicMux.HandleFunc("/students.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "templates/students.html")
	})
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/api/students", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			registrationLimiter.Middleware(http.HandlerFunc(students.Create)).ServeHTTP(w, r)
		case http.MethodGet:
			students.List(w, r)
		default:
			http.Error(w, "method not allowed", 405)
		}
	})
	adminMux.HandleFunc("/api/students/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			students.Update(w, r)
		case http.MethodDelete:
			students.Delete(w, r)
		default:
			http.Error(w, "method not allowed", 405)
		}
	})
	adminMux.Handle("/api/intake/students", registrationLimiter.Middleware(http.HandlerFunc(students.Create)))
	adminMux.HandleFunc("/api/admin/students/import", students.ImportCSV)
	adminMux.HandleFunc("/api/applications", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet { intake.ListApplications(w, r); return }
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	adminMux.HandleFunc("/api/applications/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut { intake.ReviewApplication(w, r); return }
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	adminMux.HandleFunc("/api/applications/{id}/admit", intake.AdmitApplication)
	exportHandler.Register(adminMux)
	referenceHandler.Register(adminMux)
	natsURL := getenv("NATS_URL", "nats://127.0.0.1:4222")
	nc, js, err := sharedevents.ConnectRetry(ctx, natsURL)
	if err != nil {
		log.Printf("intake service: nats connect failed: %v", err)
	} else {
		defer nc.Close()
		outbox := intakeevents.NewOutboxPublisher(db, sharedevents.NewNATSPublisher(js), 2*time.Second)
		go outbox.Run(ctx)
	}
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		corsOrigins := strings.FieldsFunc(getenv("CORS_ORIGINS", ""), func(r rune) bool { return r == ',' || r == ' ' })
		if origin != "" && slices.Contains(corsOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/healthz" || r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/logout" || (r.URL.Path == "/api/applications" && r.Method == http.MethodPost) || strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/intake-form.html" || r.URL.Path == "/students.html" {
			publicMux.ServeHTTP(w, r)
			return
		}
		intakemiddleware.RequireAdmin(db, adminMux).ServeHTTP(w, r)
	})
	secured := httpkit.SecurityHeaders(httpkit.LimitBody(1<<20, httpkit.RequestID(httpkit.Recover(httpkit.Logging(root)))))
	server := &http.Server{Addr: ":" + getenv("INTAKE_PORT", "8080"), Handler: secured, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("intake service: %v", err)
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

func requiredJWTSecret() string {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		log.Fatal("JWT_SECRET is required and must not be empty")
	}
	return secret
}
