package main

import (
	"log"
	"os"

	"CampusCanteenRank/server/internal/router"
)

func main() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-only-secret-change-me-please-1234567890"
	}

	r := router.NewEngine(secret)
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("server startup failed: %v", err)
	}
}
