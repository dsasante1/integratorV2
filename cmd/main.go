package main

import (
	"log"

	"dsasante1/integratorV2/config"
	"dsasante1/integratorV2/server"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database manager (replace with your actual DB manager)
	var dbManager interface{} // Replace with your actual DB manager

	// Create and start server
	srv := server.NewServer(cfg, dbManager)
	if err := srv.Start(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
} 