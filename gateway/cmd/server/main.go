package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/antigravity/gateway/pkg/antigravity"
	"github.com/antigravity/gateway/pkg/db"
	"github.com/antigravity/gateway/pkg/router"
	"github.com/antigravity/gateway/pkg/shield"
)

// loadEnvFile reads gateway/.env into the process. pm2 is not passing that
// file through, so token refresh was posted with an empty client id and
// Google rejected it. A switch then still rewrote the IDE and Desktop login.
func loadEnvFile() {
	candidates := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		candidates = append([]string{filepath.Join(filepath.Dir(exe), ".env")}, candidates...)
	}
	for _, path := range candidates {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			val = strings.Trim(strings.TrimSpace(val), `"'`)
			if key == "" || os.Getenv(key) != "" {
				continue
			}
			_ = os.Setenv(key, val)
		}
		_ = f.Close()
		log.Printf("Loaded environment from %s", path)
		return
	}
}

func main() {
	loadEnvFile()
	log.Println("🚀 Starting Antigravity Enterprise Gateway...")
	if os.Getenv("GOOGLE_CLIENT_ID") == "" || os.Getenv("GOOGLE_CLIENT_SECRET") == "" {
		log.Println("⚠️ GOOGLE_CLIENT_ID or GOOGLE_CLIENT_SECRET is missing; account switch cannot refresh tokens")
	}

	// PostgreSQL database connection string (OrbStack dev-postgres)
	dbConn := os.Getenv("DATABASE_URL")
	if dbConn == "" {
		dbConn = "postgres://dev:password@127.0.0.1:5432/antigravity_harness?sslmode=disable"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Database Pool
	database, err := db.New(ctx, dbConn)
	if err != nil {
		log.Fatalf("❌ Failed to connect to PostgreSQL on OrbStack: %v", err)
	}
	defer database.Close()

	// Initialize Antigravity IDE Desktop Supervisor
	supervisor := antigravity.NewSupervisor()

	// Initialize Smart Shield & Pool Rotator
	rotator := shield.NewRotator(database, supervisor)
	rotator.StartQuotaSyncWorker(ctx)

	gatewayRouter := router.NewRouter(database, rotator, supervisor)
	gatewayRouter.StartTranscriptSyncWorker(ctx)
	gatewayRouter.StartDailyReportWorker(ctx)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8050"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      gatewayRouter,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("🌐 Gateway listening on http://0.0.0.0:%s (OpenAI, Anthropic & Antigravity IDE Unified)\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down gateway gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("❌ Server forced to shutdown: %v", err)
	}

	fmt.Println("✅ Antigravity Gateway exited cleanly")
}
