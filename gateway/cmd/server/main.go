package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/antigravity/gateway/pkg/antigravity"
	"github.com/antigravity/gateway/pkg/db"
	"github.com/antigravity/gateway/pkg/router"
	"github.com/antigravity/gateway/pkg/shield"
)

func main() {
	log.Println("🚀 Starting Antigravity Enterprise Gateway...")

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
