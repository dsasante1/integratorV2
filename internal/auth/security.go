package auth

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	limiterpkg "github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

var (
	Validate    *validator.Validate
	RateLimiter *limiterpkg.Limiter
)

func InitSecurity() {
	// Initialize validator
	Validate = validator.New()
	Validate.RegisterValidation("password", validatePassword)

	// Initialize rate limiter
	// 30 requests per minute per IP
	rate := limiterpkg.Rate{
		Period: 1 * 60, // 1 minute
		Limit:  30,     // 30 requests
	}
	store := memory.NewStore()
	RateLimiter = limiterpkg.New(store, rate)
}

func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	// At least 8 characters
	if len(password) < 8 {
		return false
	}

	// At least one uppercase letter
	if !strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return false
	}

	// At least one lowercase letter
	if !strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz") {
		return false
	}

	// At least one number
	if !strings.ContainsAny(password, "0123456789") {
		return false
	}

	// At least one special character
	if !strings.ContainsAny(password, "!@#$%^&*()_+-=[]{}|;:,.<>?") {
		return false
	}

	return true
}

func ValidateEmail(email string) error {
	// Basic email format validation
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return errors.New("invalid email format")
	}

	// Additional validation rules
	if len(email) > 255 {
		return errors.New("email too long")
	}

	// Check for common disposable email domains
	disposableDomains := []string{
		"tempmail.com",
		"throwawaymail.com",
		"mailinator.com",
		// Add more as needed
	}
	for _, domain := range disposableDomains {
		if strings.HasSuffix(strings.ToLower(email), "@"+domain) {
			return errors.New("disposable email addresses are not allowed")
		}
	}

	return nil
}

func RateLimitMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ip := c.RealIP()
		context, err := RateLimiter.Get(c.Request().Context(), ip)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "rate limit error",
			})
		}

		if context.Reached {
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded",
			})
		}

		return next(c)
	}
}
