package routes

import (
	"integratorV2/internal/auth"
	"integratorV2/internal/handlers"
	"integratorV2/internal/security"

	"github.com/labstack/echo/v4"
)

func SetupRoutes(api *echo.Group) {
	// Public routes
	api.GET("/health", handlers.HealthCheck)

	// Auth routes with email validation
	authGroup := api.Group("/auth")
	authGroup.Use(security.ValidateEmail)
	authGroup.POST("/signup", handlers.Signup)
	authGroup.POST("/login", handlers.Login)

	// Protected routes
	api.Use(auth.JWTMiddleware)

	// Collection routes
	collections := api.Group("/collections")
	collections.POST("/api-key", handlers.StoreAPIKey)
	collections.POST("/api-key/rotate", handlers.RotateAPIKey)
	collections.GET("", handlers.GetCollections)
	collections.POST("/store", handlers.StoreCollection)
	collections.GET("/:id/details", handlers.GetCollectionDetails)

	// Job routes
	jobs := api.Group("/jobs")
	jobs.GET("", handlers.GetUserJobs)
	jobs.GET("/:id", handlers.GetJobStatus)
}
