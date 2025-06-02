package routes

import (
	"integratorV2/internal/auth"
	"integratorV2/internal/handlers"

	"github.com/labstack/echo/v4"
)

func SetupRoutes(e *echo.Echo) {
	// Public routes
	e.GET("/health", handlers.HealthCheck)
	e.POST("/signup", handlers.Signup)
	e.POST("/login", handlers.Login)

	// Protected routes
	api := e.Group("/api")
	api.Use(auth.JWTMiddleware)

	// Collection routes
	collections := api.Group("/collections")
	collections.POST("/api-key", handlers.StoreAPIKey)
	collections.POST("/api-key/rotate", handlers.RotateAPIKey)
	collections.GET("", handlers.GetCollections)
	collections.POST("/store", handlers.StoreCollection)
	collections.GET("/:id/details", handlers.GetCollectionDetails)
	collections.GET("/compare/:id", handlers.CompareCollections)
}
