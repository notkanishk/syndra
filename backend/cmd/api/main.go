package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"mkauth/internal/db"
	"mkauth/internal/handlers"
	"mkauth/internal/seed"
	"mkauth/internal/zitadel"
)

func main() {
	fmt.Println("MkAuth Backend Starting...")

	// Initialize connections safely (They read ENVs and connect)
	db.ConnectPostgres()
	db.ConnectRedis()

	if err := seed.EnsureDemoData(context.Background()); err != nil {
		log.Fatalf("Demo seed failed: %v", err)
	}

	// Initialize Zitadel service account auth
	if err := zitadel.InitClient(); err != nil {
		log.Printf("Zitadel Init Warning: %v", err)
	}

	mux := handlers.NewRouter()

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start server in background
	go func() {
		fmt.Println("Control Plane Backend Listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	fmt.Println("\nShutting down gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	db.PG.Close()
	if err := db.Redis.Close(); err != nil {
		log.Printf("Redis close error: %v", err)
	}

	fmt.Println("Server stopped.")
}
