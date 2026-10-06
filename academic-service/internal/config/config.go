package config

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
	NATSURL     string
	IntakeURL   string
}

func Load() Config {
	return Config{
		DatabaseURL: firstNonEmpty(os.Getenv("ACADEMIC_DATABASE_URL"), os.Getenv("DATABASE_URL")),
		JWTSecret:   requiredJWTSecret(),
		Port:        firstNonEmpty(os.Getenv("ACADEMIC_PORT"), "8082"),
		NATSURL:     firstNonEmpty(os.Getenv("NATS_URL"), "nats://127.0.0.1:4222"),
		IntakeURL:   firstNonEmpty(os.Getenv("INTAKE_URL"), "http://intake-service:8080"),
	}
}

func requiredJWTSecret() string {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		log.Fatal("JWT_SECRET is required and must not be empty")
	}
	return secret
}

func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
