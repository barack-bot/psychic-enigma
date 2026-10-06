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
	"school-platform/fees-service/internal/config"
	"school-platform/fees-service/internal/handler"
	"school-platform/fees-service/internal/middleware"
	"school-platform/shared/httpkit"
)

func main() {
	_ = config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	dsn := getenv("FEES_DATABASE_URL", getenv("DATABASE_URL", ""))
	if dsn == "" {
		log.Fatal("FEES_DATABASE_URL or DATABASE_URL is required")
	}
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	go func() { ticker := time.NewTicker(time.Hour); defer ticker.Stop(); for { select { case <-ctx.Done(): return; case <-ticker.C: _, _ = db.Exec(ctx, `DELETE FROM auth_sessions WHERE expires_at <= CURRENT_TIMESTAMP`) } } }()
	authHandler := handler.NewStudentPortalHandler(db, requiredJWTSecret())
	paymentHandler := handler.NewPaymentHandler(db)
	reportHandler := handler.NewReportHandler(db)
	mpesaHandler := handler.NewMPesaHandler(db)
	loginLimiter := httpkit.NewRateLimiter(10, time.Minute)
	publicMux := http.NewServeMux()
	intakeTarget, err := url.Parse(getenv("INTAKE_URL", "http://intake:8080"))
	if err != nil { log.Fatal(err) }
	intakeStudentsProxy := httputil.NewSingleHostReverseProxy(intakeTarget)
	publicMux.Handle("/api/auth/login", httputil.NewSingleHostReverseProxy(intakeTarget))
	publicMux.Handle("/api/intake-students", middleware.RequireFinanceAdmin(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/api/students"
		intakeStudentsProxy.ServeHTTP(w, r)
	})))
	publicMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	publicMux.Handle("/", http.FileServer(http.Dir("web")))
	publicMux.Handle("/api/student/login", loginLimiter.Middleware(http.HandlerFunc(authHandler.Login)))
	publicMux.HandleFunc("/api/student/logout", authHandler.Logout)
	publicMux.Handle("/api/student/me", middleware.RequireAuthenticated(db, http.HandlerFunc(authHandler.Me)))
	publicMux.Handle("/api/student/password", middleware.RequireAuthenticated(db, http.HandlerFunc(authHandler.UpdatePassword)))
	publicMux.Handle("/api/student/profile", middleware.RequireAuthenticated(db, http.HandlerFunc(authHandler.UpdateProfile)))
	publicMux.Handle("/api/payments/student/", middleware.RequireAuthenticated(db, http.HandlerFunc(paymentHandler.ListForStudent)))
	publicMux.Handle("/api/payments/", middleware.RequireAuthenticated(db, http.HandlerFunc(paymentHandler.Receipt)))
	publicMux.Handle("/api/reports/balance/", middleware.RequireAuthenticated(db, http.HandlerFunc(reportHandler.Balance)))
	publicMux.Handle("/api/reports/statement/", middleware.RequireAuthenticated(db, http.HandlerFunc(reportHandler.Statement)))
	publicMux.Handle("/api/reports/marks/", middleware.RequireAuthenticated(db, http.HandlerFunc(reportHandler.Marks)))
	publicMux.Handle("/api/mpesa/stk-push", middleware.RequireAuthenticated(db, http.HandlerFunc(mpesaHandler.STKPush)))
	publicMux.HandleFunc("/api/mpesa/callback", mpesaHandler.Callback)
	adminMux := http.NewServeMux()
	adminMux.Handle("/api/payments", middleware.RequireFinanceAdmin(db, http.HandlerFunc(paymentHandler.Create)))
	adminHandler := handler.NewAdminHandler(db)
	adminHandler.Register(adminMux)

	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/admin/") {
			middleware.RequireFinanceAdmin(db, adminMux).ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/payments" || r.URL.Path == "/api/reconcile" {
			middleware.RequireFinanceAdmin(db, adminMux).ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/healthz" || r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/intake-students" || strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" || r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/assets/") || r.URL.Path == "/admin" || strings.HasPrefix(r.URL.Path, "/admin/") {
			publicMux.ServeHTTP(w, r)
			return
		}
		middleware.RequireFinanceAdmin(db, adminMux).ServeHTTP(w, r)
	})
	secured := httpkit.SecurityHeaders(httpkit.LimitBody(1<<20, httpkit.RequestID(httpkit.Recover(httpkit.Logging(root)))))
	server := &http.Server{Addr: ":" + getenv("FEES_PORT", "8081"), Handler: secured, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("fees service: %v", err)
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
