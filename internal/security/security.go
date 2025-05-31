package security

import (
	"net/http"
	"regexp"
	"sync"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

var (
	// Rate limiter settings
	requestsPerMinute = 30
	rateLimiters      = make(map[string]*rate.Limiter)
	rateLimitMutex    sync.Mutex

	// Email validation regex
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

// InitSecurity initializes security features
func InitSecurity() {
	// Initialize rate limiters
	rateLimitMutex.Lock()
	defer rateLimitMutex.Unlock()
	rateLimiters = make(map[string]*rate.Limiter)
}

// RateLimiter middleware for rate limiting requests
func RateLimiter(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ip := c.RealIP()

		rateLimitMutex.Lock()
		limiter, exists := rateLimiters[ip]
		if !exists {
			limiter = rate.NewLimiter(rate.Limit(requestsPerMinute), requestsPerMinute)
			rateLimiters[ip] = limiter
		}
		rateLimitMutex.Unlock()

		if !limiter.Allow() {
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "Rate limit exceeded. Please try again later.",
			})
		}

		return next(c)
	}
}

// ValidateEmail validates email format
func ValidateEmail(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Only validate email for signup and login endpoints
		if c.Path() == "/signup" || c.Path() == "/login" {
			email := c.FormValue("email")
			if email == "" {
				email = c.QueryParam("email")
			}

			if email != "" && !emailRegex.MatchString(email) {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": "Invalid email format",
				})
			}
		}

		return next(c)
	}
}
