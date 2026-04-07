package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

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

	fmt.Println("Control Plane Backend Listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
