package main

import (
	"log"
	"net/http"

	"integratorV2/internal/auth"
	"integratorV2/internal/db"

	"github.com/labstack/echo/v4"
)

func main() {
	// Initialize database connection
	if err := db.Init(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize users table
	if err := db.InitUserTable(); err != nil {
		log.Fatalf("Failed to initialize users table: %v", err)
	}

	// Initialize security features
	auth.InitSecurity()

	e := echo.New()

	// Public routes with rate limiting
	e.POST("/signup", handleSignup, auth.RateLimitMiddleware)
	e.POST("/login", handleLogin, auth.RateLimitMiddleware)

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	e.Logger.Fatal(e.Start(":8080"))
}

func handleSignup(c echo.Context) error {
	var req auth.SignupRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate email
	if err := auth.ValidateEmail(req.Email); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Validate password
	if err := auth.Validate.Var(req.Password, "password"); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Password must be at least 8 characters long and contain uppercase, lowercase, number, and special character",
		})
	}

	// Create user
	user, err := auth.CreateUser(req.Email, req.Password)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create user"})
	}

	return c.JSON(http.StatusCreated, user)
}

func handleLogin(c echo.Context) error {
	var req auth.LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate email
	if err := auth.ValidateEmail(req.Email); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Get user by email
	user, err := auth.GetUserByEmail(req.Email)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
	}

	// Verify password
	if err := auth.VerifyPassword(user.Password, req.Password); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
	}

	// Generate token
	token, err := auth.GenerateToken(user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
	}

	return c.JSON(http.StatusOK, auth.LoginResponse{Token: token})
}
