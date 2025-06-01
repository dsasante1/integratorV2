package main

import (
	"log"

	"dsasante1/integratorV2/config"
	"dsasante1/integratorV2/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	var dbManager interface{}

	srv := server.NewServer(cfg, dbManager)
	if err := srv.Start(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
} 