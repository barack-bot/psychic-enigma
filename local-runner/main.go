package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type processSpec struct {
	name    string
	dir     string
	command string
	args    []string
	env     map[string]string
}

func main() {
	root, err := repoRoot()
	if err != nil {
		log.Fatal(err)
	}
	loadEnvFile(filepath.Join(root, ".env"))

	intakePort := nextAvailablePort("INTAKE_PORT", 8080)
	academicPort := nextAvailablePort("ACADEMIC_PORT", 8082)
	feesPort := nextAvailablePort("FEES_PORT", 8081)
	webPort := nextAvailablePort("WEB_PORT", 5500)

	if !portOpen("127.0.0.1:5432") {
		log.Println("warning: Postgres is not responding on localhost:5432. Start it before running the app stack.")
	}
	if !portOpen("127.0.0.1:4222") {
		if _, err := exec.LookPath("nats-server"); err == nil {
			log.Println("starting nats-server on localhost:4222")
			if cmd, err := startProcess(processSpec{
				name:    "nats-server",
				dir:     root,
				command: "nats-server",
				args:    []string{"-js", "-m", "8222"},
			}); err != nil {
				log.Printf("warning: failed to start nats-server: %v", err)
			} else {
				defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
				log.Println("nats-server started")
			}
		} else {
			log.Println("warning: nats-server is not installed or not reachable on localhost:4222. Install it or start it manually.")
		}
	}

	specs := []processSpec{
		{
			name:    "intake",
			dir:     filepath.Join(root, "intake-service"),
			command: "go",
			args:    []string{"run", "./cmd/api"},
			env: map[string]string{
				"INTAKE_DATABASE_URL": getenv("INTAKE_DATABASE_URL", "postgres://postgres:adminstration1587@localhost:5432/platform?sslmode=disable"),
				"DATABASE_URL":        getenv("DATABASE_URL", "postgres://postgres:adminstration1587@localhost:5432/platform?sslmode=disable"),
				"NATS_URL":            getenv("NATS_URL", "nats://127.0.0.1:4222"),
				"JWT_SECRET":          getenv("JWT_SECRET", "dev-secret"),
				"CORS_ORIGINS":       getenv("CORS_ORIGINS", fmt.Sprintf("http://localhost:%d,http://localhost:%d", academicPort, webPort)),
				"INTAKE_PORT":        strconv.Itoa(intakePort),
			},
		},
		{
			name:    "academic",
			dir:     filepath.Join(root, "academic-service"),
			command: "go",
			args:    []string{"run", "./cmd/api"},
			env: map[string]string{
				"DATABASE_URL":   getenv("ACADEMIC_DATABASE_URL", getenv("DATABASE_URL", "postgres://postgres:adminstration1587@localhost:5432/platform?sslmode=disable")),
				"INTAKE_URL":     getenv("INTAKE_URL", fmt.Sprintf("http://localhost:%d", intakePort)),
				"ACADEMIC_PORT": strconv.Itoa(academicPort),
				"JWT_SECRET":     getenv("JWT_SECRET", "dev-secret"),
			},
		},
		{
			name:    "fees",
			dir:     filepath.Join(root, "fees-service"),
			command: "go",
			args:    []string{"run", "./cmd/api"},
			env: map[string]string{
				"DATABASE_URL":          getenv("FEES_DATABASE_URL", getenv("DATABASE_URL", "postgres://postgres:adminstration1587@localhost:5432/platform?sslmode=disable")),
				"INTAKE_URL":            getenv("INTAKE_URL", fmt.Sprintf("http://localhost:%d", intakePort)),
				"JWT_SECRET":            getenv("JWT_SECRET", "dev-secret"),
				"MPESA_CALLBACK_SECRET": getenv("MPESA_CALLBACK_SECRET", "test"),
				"FEES_PORT":            strconv.Itoa(feesPort),
			},
		},
		{
			name:    "web",
			dir:     filepath.Join(root, "web"),
			command: "go",
			args:    []string{"run", "."},
			env: map[string]string{
				"SCHOOL_PORT": strconv.Itoa(webPort),
				"INTAKE_URL":  getenv("INTAKE_URL", fmt.Sprintf("http://localhost:%d", intakePort)),
				"ACADEMIC_URL": fmt.Sprintf("http://localhost:%d", academicPort),
				"FEES_URL":    fmt.Sprintf("http://localhost:%d", feesPort),
			},
		},
	}

	procs, err := startAll(specs)
	if err != nil {
		log.Fatal(err)
	}
	defer stopAll(procs)

	log.Println("all services started; press Ctrl+C to stop")
	waitForInterrupt()
	log.Println("shutting down")
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if filepath.Base(wd) == "local-runner" {
		return filepath.Dir(wd), nil
	}
	return wd, nil
}

func waitForInterrupt() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	signal.Stop(sigCh)
}

func startAll(specs []processSpec) ([]*exec.Cmd, error) {
	var mu sync.Mutex
	procs := make([]*exec.Cmd, 0, len(specs))
	var errs []string
	for _, spec := range specs {
		cmd, err := startProcess(spec)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", spec.name, err))
			continue
		}
		mu.Lock()
		procs = append(procs, cmd)
		mu.Unlock()
		log.Printf("started %s on %s", spec.name, spec.dir)
	}
	if len(errs) > 0 {
		stopAll(procs)
		return nil, errors.New(strings.Join(errs, "; "))
	}
	return procs, nil
}

func startProcess(spec processSpec) (*exec.Cmd, error) {
	cmd := exec.Command(spec.command, spec.args...)
	cmd.Dir = spec.dir
	cmd.Env = mergeEnv(spec.env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func stopAll(procs []*exec.Cmd) {
	for _, cmd := range procs {
		if cmd == nil || cmd.Process == nil {
			continue
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

func mergeEnv(extra map[string]string) []string {
	merged := map[string]string{}
	for _, item := range os.Environ() {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			merged[parts[0]] = parts[1]
		}
	}
	for key, value := range extra {
		merged[key] = value
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+merged[key])
	}
	return out
}

func portOpen(hostPort string) bool {
	conn, err := net.DialTimeout("tcp", hostPort, 1*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func nextAvailablePort(envKey string, defaultPort int) int {
	if value := os.Getenv(envKey); value != "" {
		if port, err := strconv.Atoi(value); err == nil && port > 0 {
			return port
		}
	}
	for port := defaultPort; port < defaultPort+50; port++ {
		if !portOpen("127.0.0.1:" + strconv.Itoa(port)) {
			return port
		}
	}
	return defaultPort
}

func loadEnvFile(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") && len(value) >= 2 {
			value = strings.TrimSuffix(strings.TrimPrefix(value, "\""), "\"")
		}
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func init() {
	_ = strconv.IntSize
}
