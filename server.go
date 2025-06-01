package main

import (
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

	// Initialize security features
	security.InitSecurity()

	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(security.RateLimiter)
	e.Use(security.ValidateEmail)

	// Public routes (versioned)
	v1 := e.Group("/integrator/api/v1")

	v1.GET("/health-check", handlers.HealthCheck)
	v1.POST("/signup", handlers.Signup)
	v1.POST("/login", handlers.Login)

	// Protected routes (versioned)
	protectedV1 := v1.Group("")
	protectedV1.Use(auth.JWTMiddleware)

	// User routes
	protectedV1.POST("/api-key", handlers.StoreAPIKey)
	protectedV1.GET("/collections", handlers.GetCollections)
	protectedV1.POST("/collections/store", handlers.StoreCollection)
	protectedV1.GET("/collections/:id/details", handlers.GetCollectionDetails)

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
