package security

import (
	"net/http"
	"regexp"
	"sync"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"integratorV2/internal/config"
	"integratorV2/internal/kms"
)

var (
	
	requestsPerMinute = 30
	rateLimiters      = make(map[string]*rate.Limiter)
	rateLimitMutex    sync.Mutex

	
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)


func InitSecurity() error {
	
	initRateLimiters()

	
	if err := config.InitKMS(); err != nil {
		return err
	}

	
	if err := kms.InitRotation(); err != nil {
		return err
	}

	return nil
}


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


func ValidateEmail(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		
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

func initRateLimiters() {
	rateLimitMutex.Lock()
	defer rateLimitMutex.Unlock()
	rateLimiters = make(map[string]*rate.Limiter)
}
