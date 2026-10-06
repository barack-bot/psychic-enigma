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
}

func Load() Config {
	return Config{
		DatabaseURL: firstNonEmpty(os.Getenv("FEES_DATABASE_URL"), os.Getenv("DATABASE_URL")),
		JWTSecret:   requiredJWTSecret(),
		Port:        firstNonEmpty(os.Getenv("FEES_PORT"), "8081"),
		NATSURL:     firstNonEmpty(os.Getenv("NATS_URL"), "nats://127.0.0.1:4222"),
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
