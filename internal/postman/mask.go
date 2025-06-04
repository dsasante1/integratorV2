package postman

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// SensitiveDataPatterns defines regex patterns for sensitive data detection
var SensitiveDataPatterns = map[string]*regexp.Regexp{
	"api_key":     regexp.MustCompile(`(?i)(api[_-]?key|apikey|x-api-key)`),
	"auth_token":  regexp.MustCompile(`(?i)(auth[_-]?token|bearer|jwt|token)`),
	"password":    regexp.MustCompile(`(?i)(password|passwd|pwd)`),
	"secret":      regexp.MustCompile(`(?i)(secret|private[_-]?key|client[_-]?secret)`),
	"credential":  regexp.MustCompile(`(?i)(credential|cred)`),
	"access_key":  regexp.MustCompile(`(?i)(access[_-]?key|accesskey)`),
	"session":     regexp.MustCompile(`(?i)(session[_-]?id|sessionid|jsessionid)`),
	"oauth":       regexp.MustCompile(`(?i)(oauth|refresh[_-]?token)`),
	"email":       regexp.MustCompile(`(?i)[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
	"phone":       regexp.MustCompile(`(?i)(\+?[0-9]{1,3}[-. ]?)?\(?[0-9]{3}\)?[-. ]?[0-9]{3}[-. ]?[0-9]{4}`),
	"credit_card": regexp.MustCompile(`(?i)(\d{4}[- ]?){3}\d{4}`),
	"ssn":         regexp.MustCompile(`(?i)\d{3}-?\d{2}-?\d{4}`),
	"ip_address":  regexp.MustCompile(`(?i)\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`),
}

// SensitiveValuePatterns detects sensitive values regardless of field name
var SensitiveValuePatterns = map[string]*regexp.Regexp{
	"jwt_token":    regexp.MustCompile(`^eyJ[A-Za-z0-9-_=]+\.[A-Za-z0-9-_=]+\.?[A-Za-z0-9-_.+/=]*$`),
	"bearer_token": regexp.MustCompile(`^Bearer\s+[A-Za-z0-9\-\._~\+\/]+=*$`),
	"api_key_like": regexp.MustCompile(`^[A-Za-z0-9]{20,}$`),         // Long alphanumeric strings
	"base64_long":  regexp.MustCompile(`^[A-Za-z0-9+/]{40,}={0,2}$`), // Long base64 strings
}

// CollectionValidator handles validation of Postman collections
type CollectionValidator struct {
	validate *validator.Validate
}

// MaskingStats tracks what was masked during the process
type MaskingStats struct {
	FieldsMasked     int            `json:"fields_masked"`
	ValuesMasked     int            `json:"values_masked"`
	MaskedFieldTypes map[string]int `json:"masked_field_types"`
	Warnings         []string       `json:"warnings,omitempty"`
}

// MaskingResult contains the masked collection and statistics
type MaskingResult struct {
	Collection *PostmanCollectionResponse `json:"collection"`
	Stats      MaskingStats               `json:"stats"`
}

// NewCollectionValidator creates a new validator instance
func NewCollectionValidator() *CollectionValidator {
	v := validator.New()

	// Register custom validation for sensitive fields (optional)
	v.RegisterValidation("no_sensitive_data", validateNoSensitiveData)

	return &CollectionValidator{
		validate: v,
	}
}

// validateNoSensitiveData custom validator to warn about potential sensitive data
func validateNoSensitiveData(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	return !containsSensitiveValue(value)
}

// ValidateCollection validates the collection but allows missing metadata
func (cv *CollectionValidator) ValidateCollection(collection *PostmanCollectionResponse) error {
	if collection == nil {
		return fmt.Errorf("collection cannot be nil")
	}

	if collection.Collection.Item == nil {
		return fmt.Errorf("collection items cannot be nil")
	}

	// Don't fail on missing ID or Name - these are metadata fields
	// Focus on validating the structure needed for masking
	return nil
}

// IsSensitiveField checks if a field name indicates sensitive data
func IsSensitiveField(fieldName string) bool {
	fieldName = strings.ToLower(fieldName)
	for _, pattern := range SensitiveDataPatterns {
		if pattern.MatchString(fieldName) {
			return true
		}
	}
	return false
}

// containsSensitiveValue checks if a value looks like sensitive data
func containsSensitiveValue(value string) bool {
	if len(value) < 8 { // Skip very short values
		return false
	}

	for _, pattern := range SensitiveValuePatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

// MaskValue masks a sensitive value based on its length and type
func MaskValue(value string) string {
	if value == "" {
		return value
	}

	length := len(value)

	switch {
	case length <= 4:
		return "****"
	case length <= 8:
		return value[:2] + "****"
	case length <= 16:
		return value[:3] + "********" + value[length-3:]
	default:
		// For long values like API keys, show more context
		return value[:4] + "..." + strings.Repeat("*", 8) + "..." + value[length-4:]
	}
}

// MaskEmail masks an email address preserving domain for debugging
func MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "****@****.***"
	}

	username := parts[0]
	domain := parts[1]

	if len(username) <= 2 {
		return "**@" + domain
	}

	maskedUsername := username[:1] + strings.Repeat("*", len(username)-1)
	return maskedUsername + "@" + domain
}

// MaskPhone masks a phone number
func MaskPhone(phone string) string {
	// Remove common separators to get clean number
	cleaned := regexp.MustCompile(`[^\d+]`).ReplaceAllString(phone, "")

	if len(cleaned) <= 6 {
		return "***-***-****"
	}

	// Keep country code if present and last 4 digits
	if strings.HasPrefix(cleaned, "+") && len(cleaned) > 10 {
		return cleaned[:2] + "***-***-" + cleaned[len(cleaned)-4:]
	}

	return "***-***-" + cleaned[len(cleaned)-4:]
}

// MaskCreditCard masks a credit card number
func MaskCreditCard(card string) string {
	// Remove spaces and dashes
	cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(card, "")

	if len(cleaned) < 13 || len(cleaned) > 19 {
		return "****-****-****-****"
	}

	// Keep last 4 digits
	return "****-****-****-" + cleaned[len(cleaned)-4:]
}

// MaskURL masks sensitive parts of URLs while preserving structure
func MaskURL(url string) string {
	// Mask query parameters that might contain sensitive data
	if strings.Contains(url, "?") {
		parts := strings.Split(url, "?")
		baseURL := parts[0]

		// Check if query params contain sensitive data
		queryParams := parts[1]
		if IsSensitiveField(queryParams) || containsSensitiveValue(queryParams) {
			return baseURL + "?[MASKED_PARAMS]"
		}
	}

	return url
}

// determineMaskingStrategy decides how to mask a value based on field name and content
func determineMaskingStrategy(fieldName, value string) (string, string) {
	fieldLower := strings.ToLower(fieldName)

	switch {
	case strings.Contains(fieldLower, "email"):
		return MaskEmail(value), "email"
	case strings.Contains(fieldLower, "phone"):
		return MaskPhone(value), "phone"
	case strings.Contains(fieldLower, "credit") || strings.Contains(fieldLower, "card"):
		return MaskCreditCard(value), "credit_card"
	case strings.Contains(fieldLower, "url") || strings.Contains(fieldLower, "endpoint"):
		return MaskURL(value), "url"
	case strings.Contains(fieldLower, "token") || strings.Contains(fieldLower, "jwt"):
		return MaskValue(value), "token"
	case strings.Contains(fieldLower, "key"):
		return MaskValue(value), "api_key"
	case strings.Contains(fieldLower, "password") || strings.Contains(fieldLower, "secret"):
		return "********", "credential"
	default:
		return MaskValue(value), "sensitive_data"
	}
}

// MaskSensitiveData recursively processes data structure and masks sensitive values
func MaskSensitiveData(data interface{}, stats *MaskingStats) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		masked := make(map[string]interface{})

		for key, value := range v {
			switch val := value.(type) {
			case string:
				if IsSensitiveField(key) || containsSensitiveValue(val) {
					maskedValue, dataType := determineMaskingStrategy(key, val)
					masked[key] = maskedValue

					// Update statistics
					stats.FieldsMasked++
					stats.ValuesMasked++
					if stats.MaskedFieldTypes == nil {
						stats.MaskedFieldTypes = make(map[string]int)
					}
					stats.MaskedFieldTypes[dataType]++

					slog.Info("Masked field", "type", dataType, "key", key)
				} else {
					masked[key] = val
				}
			default:
				masked[key] = MaskSensitiveData(value, stats)
			}
		}
		return masked

	case []interface{}:
		masked := make([]interface{}, len(v))
		for i, value := range v {
			masked[i] = MaskSensitiveData(value, stats)
		}
		return masked

	default:
		return data
	}
}

// MaskCollection masks sensitive data in a Postman collection
func MaskCollection(collection *PostmanCollectionResponse) (*MaskingResult, error) {
	if collection == nil {
		return nil, fmt.Errorf("collection cannot be nil")
	}

	// Initialize statistics
	stats := MaskingStats{
		FieldsMasked:     0,
		ValuesMasked:     0,
		MaskedFieldTypes: make(map[string]int),
		Warnings:         []string{},
	}

	// Validate collection structure
	validator := NewCollectionValidator()
	if err := validator.ValidateCollection(collection); err != nil {
		return nil, fmt.Errorf("collection validation failed: %w", err)
	}

	// Create a deep copy of the collection
	maskedCollection := *collection

	// Set default values for missing metadata to prevent validation issues
	if maskedCollection.Collection.ID == "" {
		maskedCollection.Collection.ID = "masked-collection"
		stats.Warnings = append(stats.Warnings, "Collection ID was empty, set to default")
	}

	if maskedCollection.Collection.Name == "" {
		maskedCollection.Collection.Name = "Masked Collection"
		stats.Warnings = append(stats.Warnings, "Collection name was empty, set to default")
	}

	// Process collection items
	if collection.Collection.Item != nil {
		var items []interface{}
		if err := json.Unmarshal(collection.Collection.Item, &items); err != nil {
			return nil, fmt.Errorf("failed to parse collection items: %w", err)
		}

		// Mask sensitive data in items
		maskedItems := MaskSensitiveData(items, &stats)

		// Convert back to JSON
		maskedItemsJSON, err := json.Marshal(maskedItems)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal masked items: %w", err)
		}
		maskedCollection.Collection.Item = maskedItemsJSON
	}

	// Process collection info
	if collection.Collection.Info != nil {
		var info map[string]interface{}
		if err := json.Unmarshal(collection.Collection.Info, &info); err != nil {
			return nil, fmt.Errorf("failed to parse collection info: %w", err)
		}

		maskedInfo := MaskSensitiveData(info, &stats)
		maskedInfoJSON, err := json.Marshal(maskedInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal masked info: %w", err)
		}
		maskedCollection.Collection.Info = maskedInfoJSON
	}

	// Handle collection variables
	var variables []interface{}
	if err := json.Unmarshal(collection.Collection.Info, &variables); err != nil {
		slog.Warn("Failed to parse collection variables", "error", err)
		stats.Warnings = append(stats.Warnings, "Failed to parse collection variables")
	} else {
		maskedVariables := MaskSensitiveData(variables, &stats)
		maskedVariablesJSON, err := json.Marshal(maskedVariables)
		if err != nil {
			slog.Warn("Failed to marshal masked variables", "error", err)
			stats.Warnings = append(stats.Warnings, "Failed to marshal masked variables")
		} else {
			maskedCollection.Collection.Info = maskedVariablesJSON
		}
	}

	slog.Info("Masking completed", "fields_masked", stats.FieldsMasked, "values_processed", stats.ValuesMasked)

	return &MaskingResult{
		Collection: &maskedCollection,
		Stats:      stats,
	}, nil
}
