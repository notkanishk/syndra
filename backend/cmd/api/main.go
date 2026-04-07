package main

import (
	"fmt"
	"log"
	"net/http"

	"mkauth/internal/db"
	"mkauth/internal/handlers"
	"mkauth/internal/zitadel"
)

func main() {
	fmt.Println("MkAuth Backend Starting...")

	// Initialize connections safely (They read ENVs and connect)
	db.ConnectPostgres()
	db.ConnectRedis()

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
