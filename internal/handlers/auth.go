package handlers

import (
	"net/http"
	"strings"

	"integratorV2/internal/auth"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

func Signup(c echo.Context) error {
	var req auth.SignupRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate the entire struct
	if err := auth.Validate.Struct(&req); err != nil {
		// Handle validation errors
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			// Get the first validation error
			if len(validationErrors) > 0 {
				fieldError := validationErrors[0]
				switch fieldError.Tag() {
				case "required":
					return c.JSON(http.StatusBadRequest, map[string]string{"error": fieldError.Field() + " is required"})
				case "email":
					return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid email format"})
				case "password":
					return c.JSON(http.StatusBadRequest, map[string]string{
						"error": "Password must be at least 8 characters long and contain uppercase, lowercase, number, and special character",
					})
				default:
					return c.JSON(http.StatusBadRequest, map[string]string{"error": "Validation failed"})
				}
			}
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Validation failed"})
	}

	// Additional email validation (if you have custom logic)
	if err := auth.ValidateEmail(req.Email); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Create user
	user, err := auth.CreateUser(req.Email, req.Password)
	if err != nil {
		// Check for duplicate email error
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return c.JSON(http.StatusConflict, map[string]string{"error": "Email already exists"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create user"})
	}

	return c.JSON(http.StatusCreated, user)
}

func Login(c echo.Context) error {
	var req auth.LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate the request
	if err := auth.Validate.Struct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid email or password format"})
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
