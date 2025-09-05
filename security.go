package main

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
	validate    *validator.Validate
	rateLimiter *limiterpkg.Limiter
)

func initSecurity() {
	
	validate = validator.New()
	validate.RegisterValidation("password", validatePassword)

	
	
	rate := limiterpkg.Rate{
		Period: 1 * 60, // 1 minute
		Limit:  30,     // 30 requests
	}
	store := memory.NewStore()
	rateLimiter = limiterpkg.New(store, rate)
}

func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	
	if len(password) < 8 {
		return false
	}

	
	if !strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return false
	}

	
	if !strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz") {
		return false
	}

	
	if !strings.ContainsAny(password, "0123456789") {
		return false
	}

	
	if !strings.ContainsAny(password, "!@#$%^&*()_+-=[]{}|;:,.<>?") {
		return false
	}

	return true
}

func validateEmail(email string) error {
	
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return errors.New("invalid email format")
	}

	
	if len(email) > 255 {
		return errors.New("email too long")
	}

	
	disposableDomains := []string{
		"tempmail.com",
		"throwawaymail.com",
		"mailinator.com",
		
	}
	for _, domain := range disposableDomains {
		if strings.HasSuffix(strings.ToLower(email), "@"+domain) {
			return errors.New("disposable email addresses are not allowed")
		}
	}

	return nil
}

func rateLimitMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ip := c.RealIP()
		context, err := rateLimiter.Get(c.Request().Context(), ip)
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
