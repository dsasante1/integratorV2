package main

import (
	"log"

	"integratorV2/internal/auth"
	"integratorV2/internal/db"
	"integratorV2/internal/handlers"
	"integratorV2/internal/security"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Initialize database
	db.InitDB()

	// Initialize collection tables
	if err := db.InitCollectionTables(); err != nil {
		log.Fatalf("Failed to initialize collection tables: %v", err)
	}

	// Initialize security features
	security.InitSecurity()

	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(security.RateLimiter)
	e.Use(security.ValidateEmail)

	// Public routes
	e.GET("/health-check", handlers.HealthCheck)
	e.POST("/signup", handlers.Signup)
	e.POST("/login", handlers.Login)

	// Protected routes
	api := e.Group("/api")
	api.Use(auth.JWTMiddleware)

	// User routes
	api.POST("/api-key", handlers.StoreAPIKey)
	api.GET("/collections", handlers.GetCollections)
	api.GET("/collections/:id", handlers.GetCollection)
	api.GET("/collections/:id/history", handlers.GetCollectionHistory)
	api.GET("/collections/:id/changes", handlers.GetCollectionChanges)

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
