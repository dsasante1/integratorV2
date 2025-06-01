package auth

import (
	"net/http"

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
