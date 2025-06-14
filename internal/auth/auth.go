package auth

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"regexp"
	"time"

	"integratorV2/internal/db"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)


func init() {
	Validate = validator.New()
	
	// Register custom password validation
	Validate.RegisterValidation("password", validatePassword)
}

// Custom password validation function
func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	
	// At least 8 characters
	if len(password) < 8 {
		return false
	}
	
	// Contains uppercase letter
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	// Contains lowercase letter
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	// Contains digit
	hasDigit := regexp.MustCompile(`\d`).MatchString(password)
	// Contains special character
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password)
	
	return hasUpper && hasLower && hasDigit && hasSpecial
}

// ValidateEmail validates email format and requirements
func ValidateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}
	
	// Use validator to check email format
	if err := Validate.Var(email, "required,email"); err != nil {
		return errors.New("invalid email format")
	}
	
	return nil
}

type User struct {
	ID        int64     `db:"id" json:"id"`
	Email     string    `db:"email" json:"email" validate:"required,email"`
	Password  string    `db:"password" json:"-" validate:"required,password"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type SignupRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,password"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

func CreateUser(email, password string) (*User, error) {
	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Insert the user
	user := &User{
		Email:    email,
		Password: string(hashedPassword),
	}

	err = db.DB.QueryRow(`
		INSERT INTO users (email, password)
		VALUES ($1, $2)
		RETURNING id, created_at
	`, email, hashedPassword).Scan(&user.ID, &user.CreatedAt)

	if err != nil {
		return nil, err
	}

	return user, nil
}

func GetUserByEmail(email string) (*User, error) {
	user := &User{}
	err := db.DB.Get(user, "SELECT * FROM users WHERE email = $1", email)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func GenerateToken(user *User) (string, error) {
	// Create the Claims
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "your-secret-key" // Use this only for development
	}

	return token.SignedString([]byte(secret))
}

func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

func JWTMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authorization header is required"})
		}

		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token format"})
		}
		
		tokenString := authHeader[7:]
		if tokenString == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token format"})
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			
			secret := os.Getenv("JWT_SECRET")
			if secret == "" {
				secret = "your-secret-key" 
			}
			
			return []byte(secret), nil
		})

		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {

			userID := int64(claims["user_id"].(float64))
			c.Set("user_id", userID)
			return next(c)
		}

		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
	}
}