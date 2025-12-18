package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
)

func main() {
	// TODO: Implement full dependency injection
	// For now, just start a basic server

	port := ":8080"
	fmt.Printf("Starting API server on %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Server failed:", err)
	}
}

func setupDatabase() (*sql.DB, error) {
	// Database connection setup
	return nil, fmt.Errorf("not implemented")
}
