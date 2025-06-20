package postman

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
)


type PostmanCollectionStructure struct {
	Info  CollectionInfo   `json:"info"`
	Item  []CollectionItem `json:"item"`
	Event []interface{}    `json:"event,omitempty"`
	Variable []interface{} `json:"variable,omitempty"`
}


type CollectionInfo struct {
	PostmanID  string `json:"_postman_id"`
	Name       string `json:"name"`
	Schema     string `json:"schema"`
	ExporterID string `json:"_exporter_id,omitempty"`
}


type CollectionItem struct {
	Name     string           `json:"name"`
	Request  *Request         `json:"request,omitempty"`
	Response []Response       `json:"response,omitempty"`
	Item     []CollectionItem `json:"item,omitempty"` // For folders
}


type Request struct {
	Method string      `json:"method"`
	Header []Header    `json:"header"`
	Body   *Body       `json:"body,omitempty"`
	URL    interface{} `json:"url"` // Can be string or URLObject
}


type Body struct {
	Mode    string          `json:"mode"`
	Raw     string          `json:"raw,omitempty"`
	Options json.RawMessage `json:"options,omitempty"`
}


type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}


type Response struct {
	Name            string          `json:"name"`
	OriginalRequest *Request        `json:"originalRequest,omitempty"`
	Status          string          `json:"status"`
	Code            int             `json:"code"`
	Header          []Header        `json:"header"`
	Cookie          []interface{}   `json:"cookie"`
	Body            string          `json:"body"`
}


type MaskingConfig struct {
	Enabled              bool                      `json:"enabled"`
	SkipPaths            []string                  `json:"skip_paths"`
	CustomPatterns       map[string]string         `json:"custom_patterns"`
	PreserveMaskedValues bool                      `json:"preserve_masked_values"`
	MaskingKey           string                    `json:"-"`
	AllowPartialMasking  bool                      `json:"allow_partial_masking"`
	LogMasking           bool                      `json:"log_masking"`
	MaskRequestBodies    bool                      `json:"mask_request_bodies"`
	MaskResponseBodies   bool                      `json:"mask_response_bodies"`
	MaskHeaders          bool                      `json:"mask_headers"`
	customRegexps        map[string]*regexp.Regexp
}


func DefaultMaskingConfig() *MaskingConfig {
	return &MaskingConfig{
		Enabled:              true,
		SkipPaths:            []string{},
		PreserveMaskedValues: false,
		AllowPartialMasking:  true,
		LogMasking:           false,
		MaskRequestBodies:    true,
		MaskResponseBodies:   true,
		MaskHeaders:          false,
		customRegexps:        make(map[string]*regexp.Regexp),
	}
}


type MaskedValue struct {
	Masked    string  `json:"masked"`
	Encrypted *string `json:"encrypted,omitempty"`
	Type      string  `json:"type"`
	Path      string  `json:"path"`
}





type MaskingResult struct {
	Collection *PostmanCollectionStructure `json:"collection"`
}


type PatternRegistry struct {
	fieldPatterns map[string]*regexp.Regexp
	valuePatterns map[string]*regexp.Regexp
	mu            sync.RWMutex
}

var patternRegistry *PatternRegistry
var registryOnce sync.Once


func initPatternRegistry() {
	registryOnce.Do(func() {
		patternRegistry = &PatternRegistry{
			fieldPatterns: make(map[string]*regexp.Regexp),
			valuePatterns: make(map[string]*regexp.Regexp),
		}
		
		
		fieldPatterns := map[string]string{
			"api_key":     `(?i)(api[_-]?key|apikey|x-api-key)`,
			"auth_token":  `(?i)(auth[_-]?token|bearer|jwt|token)`,
			"password":    `(?i)(password|passwd|pwd)`,
			"secret":      `(?i)(secret|private[_-]?key|client[_-]?secret)`,
			"credential":  `(?i)(credential|cred)`,
			"access_key":  `(?i)(access[_-]?key|accesskey)`,
			"session":     `(?i)(session[_-]?id|sessionid|jsessionid)`,
			"oauth":       `(?i)(oauth|refresh[_-]?token)`,
			"email":       `(?i)(email|e-mail|mail)`,
		}
		
		for name, pattern := range fieldPatterns {
			patternRegistry.fieldPatterns[name] = regexp.MustCompile(pattern)
		}
		
		
		valuePatterns := map[string]string{
			"email":       `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
			"phone":       `(\+?[0-9]{1,3}[-. ]?)?\(?[0-9]{3}\)?[-. ]?[0-9]{3}[-. ]?[0-9]{4}`,
			"credit_card": `(\d{4}[- ]?){3}\d{4}`,
			"ssn":         `\d{3}-?\d{2}-?\d{4}`,
			"ip_address":  `\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`,
			"jwt_token":   `eyJ[A-Za-z0-9-_=]+\.[A-Za-z0-9-_=]+\.?[A-Za-z0-9-_.+/=]*`,
			"bearer_token": `Bearer\s+[A-Za-z0-9\-\._~\+\/]+=*`,
			"api_key_like": `[A-Za-z0-9]{32,}`,
			"base64_long":  `[A-Za-z0-9+/]{40,}={0,2}`,
		}
		
		for name, pattern := range valuePatterns {
			patternRegistry.valuePatterns[name] = regexp.MustCompile(pattern)
		}
	})
}


type CollectionMasker struct {
	config    *MaskingConfig
	validator *validator.Validate
	cipher    cipher.AEAD
}


func NewCollectionMasker(config *MaskingConfig) (*CollectionMasker, error) {
	if config == nil {
		config = DefaultMaskingConfig()
	}
	
	initPatternRegistry()
	
	
	if config.CustomPatterns != nil {
		config.customRegexps = make(map[string]*regexp.Regexp)
		for name, pattern := range config.CustomPatterns {
			regex, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid custom pattern %s: %w", name, err)
			}
			config.customRegexps[name] = regex
		}
	}
	
	masker := &CollectionMasker{
		config:    config,
		validator: validator.New(),
	}
	
	
	if config.PreserveMaskedValues && config.MaskingKey != "" {
		cipher, err := createCipher(config.MaskingKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create cipher: %w", err)
		}
		masker.cipher = cipher
	}
	
	return masker, nil
}


func (m *CollectionMasker) MaskCollection(collection *PostmanCollectionStructure) (*MaskingResult, error) {
	if !m.config.Enabled {
		return &MaskingResult{
			Collection: collection,
			// Stats:      &MaskingStats{},
			// MaskedAt:   time.Now(),
		}, nil
	}
	
	// start := time.Now()
	
	if collection == nil {
		return nil, fmt.Errorf("collection cannot be nil")
	}
	
	
	// stats := &MaskingStats{
	// 	MaskedFieldTypes: make(map[string]int),
	// 	MaskedValues:     make(map[string]*MaskedValue),
	// }
	
	
	maskedCollection := &PostmanCollectionStructure{}
	*maskedCollection = *collection
	
	
	ctx := &maskingContext{
		masker: m,
		// stats:  stats,
	}
	
	
	maskedCollection.Item = m.maskItems(ctx, collection.Item, "item")
	

	
	if m.config.LogMasking {
		slog.Info("Masking completed",
)
	}
	
	return &MaskingResult{
		Collection: maskedCollection,

	}, nil
}


type maskingContext struct {
	masker *CollectionMasker

}


func (m *CollectionMasker) maskItems(ctx *maskingContext, items []CollectionItem, path string) []CollectionItem {
	maskedItems := make([]CollectionItem, len(items))
	
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		maskedItems[i] = m.maskItem(ctx, item, itemPath)
	}
	
	return maskedItems
}


func (m *CollectionMasker) maskItem(ctx *maskingContext, item CollectionItem, path string) CollectionItem {
	maskedItem := item
	
	
	if item.Request != nil && m.config.MaskRequestBodies {
		maskedItem.Request = m.maskRequest(ctx, item.Request, path+".request")
	}
	
	
	if len(item.Response) > 0 && m.config.MaskResponseBodies {
		maskedItem.Response = m.maskResponses(ctx, item.Response, path+".response")
	}
	
	
	if len(item.Item) > 0 {
		maskedItem.Item = m.maskItems(ctx, item.Item, path+".item")
	}
	
	return maskedItem
}


func (m *CollectionMasker) maskRequest(ctx *maskingContext, request *Request, path string) *Request {
	maskedRequest := &Request{}
	*maskedRequest = *request
	
	
	if m.config.MaskHeaders && len(request.Header) > 0 {
		maskedRequest.Header = m.maskHeaders(ctx, request.Header, path+".header")
	}
	
	
	if request.Body != nil && request.Body.Raw != "" {
		maskedBody := m.maskRawBody(ctx, request.Body.Raw, path+".body.raw")
		maskedRequest.Body = &Body{
			Mode:    request.Body.Mode,
			Raw:     maskedBody,
			Options: request.Body.Options,
		}
	}
	
	return maskedRequest
}


func (m *CollectionMasker) maskResponses(ctx *maskingContext, responses []Response, path string) []Response {
	maskedResponses := make([]Response, len(responses))
	
	for i, response := range responses {
		respPath := fmt.Sprintf("%s[%d]", path, i)
		maskedResponses[i] = m.maskResponse(ctx, response, respPath)
	}
	
	return maskedResponses
}


func (m *CollectionMasker) maskResponse(ctx *maskingContext, response Response, path string) Response {
	maskedResponse := response
	
	
	if m.config.MaskHeaders && len(response.Header) > 0 {
		maskedResponse.Header = m.maskHeaders(ctx, response.Header, path+".header")
	}
	
	
	if response.Body != "" {
		maskedResponse.Body = m.maskRawBody(ctx, response.Body, path+".body")
	}
	
	
	if response.OriginalRequest != nil {
		maskedResponse.OriginalRequest = m.maskRequest(ctx, response.OriginalRequest, path+".originalRequest")
	}
	
	return maskedResponse
}


func (m *CollectionMasker) maskHeaders(ctx *maskingContext, headers []Header, path string) []Header {
	maskedHeaders := make([]Header, len(headers))
	
	for i, header := range headers {
		headerPath := fmt.Sprintf("%s[%d]", path, i)
		maskedHeaders[i] = Header{
			Key:   header.Key,
			Value: m.maskHeaderValue(ctx, header.Key, header.Value, headerPath),
		}
	}
	
	return maskedHeaders
}


func (m *CollectionMasker) maskHeaderValue(ctx *maskingContext, key, value, path string) string {
	
	if m.isSensitiveField(key) {
		masked := m.applyMaskingStrategy("header", value)
		m.recordMaskedValue(ctx, path, key, value, masked, "header")
		return masked
	}
	
	
	if dataType, isSensitive := m.detectSensitiveValue(value); isSensitive {
		masked := m.applyMaskingStrategy(dataType, value)
		m.recordMaskedValue(ctx, path, key, value, masked, dataType)
		return masked
	}
	
	return value
}


func (m *CollectionMasker) maskRawBody(ctx *maskingContext, body, path string) string {
	
	var jsonData interface{}
	if err := json.Unmarshal([]byte(body), &jsonData); err == nil {
		
		maskedData := m.maskJSON(ctx, jsonData, path)
		maskedJSON, _ := json.MarshalIndent(maskedData, "", "    ")
		return string(maskedJSON)
	}
	
	
	if dataType, isSensitive := m.detectSensitiveValue(body); isSensitive {
		masked := m.applyMaskingStrategy(dataType, body)
		m.recordMaskedValue(ctx, path, "raw", body, masked, dataType)
		return masked
	}
	
	return body
}


func (m *CollectionMasker) maskJSON(ctx *maskingContext, data interface{}, path string) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		masked := make(map[string]interface{})
		for key, value := range v {
			keyPath := path + "." + key
			maskedValue := m.maskJSON(ctx, value, keyPath)
			
			
			if strVal, ok := value.(string); ok && m.isSensitiveField(key) {
				masked[key] = m.maskSensitiveValue(ctx, key, strVal, keyPath)
			} else {
				masked[key] = maskedValue
			}
		}
		return masked
		
	case []interface{}:
		masked := make([]interface{}, len(v))
		for i, item := range v {
			indexPath := fmt.Sprintf("%s[%d]", path, i)
			masked[i] = m.maskJSON(ctx, item, indexPath)
		}
		return masked
		
	case string:
		
		if dataType, isSensitive := m.detectSensitiveValue(v); isSensitive {
			masked := m.applyMaskingStrategy(dataType, v)
			m.recordMaskedValue(ctx, path, "value", v, masked, dataType)
			return masked
		}
		return v
		
	default:
		return data
	}
}


func (m *CollectionMasker) maskSensitiveValue(ctx *maskingContext, fieldName, value, path string) string {
	dataType := m.getFieldType(fieldName)
	masked := m.applyMaskingStrategy(dataType, value)
	m.recordMaskedValue(ctx, path, fieldName, value, masked, dataType)
	return masked
}


func (m *CollectionMasker) recordMaskedValue(ctx *maskingContext, path, fieldName, original, masked, dataType string) {
	maskedValue := &MaskedValue{
		Masked: masked,
		Type:   dataType,
		Path:   path,
	}
	
	
	if m.config.PreserveMaskedValues && m.cipher != nil {
		encrypted, err := m.encryptValue(original)
		if err == nil {
			maskedValue.Encrypted = &encrypted
		}
	}

	
	if m.config.LogMasking {
		slog.Debug("Masked sensitive data",
			"path", path,
			"type", dataType,
			"field", fieldName)
	}
}


func (m *CollectionMasker) isSensitiveField(fieldName string) bool {
	fieldLower := strings.ToLower(fieldName)
	
	
	patternRegistry.mu.RLock()
	for _, pattern := range patternRegistry.fieldPatterns {
		if pattern.MatchString(fieldLower) {
			patternRegistry.mu.RUnlock()
			return true
		}
	}
	patternRegistry.mu.RUnlock()
	
	
	for _, pattern := range m.config.customRegexps {
		if pattern.MatchString(fieldName) {
			return true
		}
	}
	
	return false
}


func (m *CollectionMasker) detectSensitiveValue(value string) (string, bool) {
	if len(value) < 3 { 
		return "", false
	}
	
	patternRegistry.mu.RLock()
	defer patternRegistry.mu.RUnlock()
	
	for dataType, pattern := range patternRegistry.valuePatterns {
		if pattern.MatchString(value) {
			return dataType, true
		}
	}
	
	return "", false
}


func (m *CollectionMasker) getFieldType(fieldName string) string {
	fieldLower := strings.ToLower(fieldName)
	
	switch {
	case strings.Contains(fieldLower, "email"):
		return "email"
	case strings.Contains(fieldLower, "phone"):
		return "phone"
	case strings.Contains(fieldLower, "credit") || strings.Contains(fieldLower, "card"):
		return "credit_card"
	case strings.Contains(fieldLower, "token") || strings.Contains(fieldLower, "jwt"):
		return "token"
	case strings.Contains(fieldLower, "key"):
		return "api_key"
	case strings.Contains(fieldLower, "password"):
		return "password"
	case strings.Contains(fieldLower, "secret"):
		return "secret"
	default:
		return "sensitive_data"
	}
}


func (m *CollectionMasker) applyMaskingStrategy(dataType, value string) string {
	if !m.config.AllowPartialMasking {
		return "********"
	}
	
	switch dataType {
	case "email":
		return maskEmail(value)
	case "phone":
		return maskPhone(value)
	case "credit_card":
		return maskCreditCard(value)
	case "jwt_token", "bearer_token", "token":
		return maskToken(value)
	case "api_key", "api_key_like":
		return maskAPIKey(value)
	case "password":
		return "********"
	case "ip_address":
		return maskIPAddress(value)
	case "header":
		return "****"
	default:
		return maskGeneric(value)
	}
}


func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "****@****.***"
	}
	
	username := parts[0]
	domain := parts[1]
	
	if len(username) <= 2 {
		return "**@" + domain
	}
	
	return username[:1] + strings.Repeat("*", len(username)-2) + username[len(username)-1:] + "@" + domain
}

func maskPhone(phone string) string {
	cleaned := regexp.MustCompile(`[^\d+]`).ReplaceAllString(phone, "")
	
	if len(cleaned) <= 6 {
		return "***-***-****"
	}
	
	if strings.HasPrefix(cleaned, "+") && len(cleaned) > 10 {
		return cleaned[:3] + "***-***-" + cleaned[len(cleaned)-4:]
	}
	
	return "***-***-" + cleaned[len(cleaned)-4:]
}

func maskCreditCard(card string) string {
	cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(card, "")
	
	if len(cleaned) < 13 || len(cleaned) > 19 {
		return "****-****-****-****"
	}
	
	return "**** **** **** " + cleaned[len(cleaned)-4:]
}

func maskToken(token string) string {
	if len(token) <= 20 {
		return "****"
	}
	
	
	if strings.HasPrefix(token, "Bearer ") {
		return "Bearer ****"
	}
	
	if strings.HasPrefix(token, "eyJ") { 
		parts := strings.Split(token, ".")
		if len(parts) == 3 {
			return "eyJ****." + "****." + "****"
		}
	}
	
	return token[:4] + "****" + token[len(token)-4:]
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "********"
	}
	
	
	prefixLen := 4
	suffixLen := 4
	
	if len(key) <= prefixLen+suffixLen+4 {
		return key[:prefixLen] + "****"
	}
	
	return key[:prefixLen] + "..." + strings.Repeat("*", 8) + "..." + key[len(key)-suffixLen:]
}

func maskIPAddress(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return "***.***.***.***"
	}
	
	
	return parts[0] + ".***.***." + parts[3]
}

func maskGeneric(value string) string {
	length := len(value)
	
	switch {
	case length <= 4:
		return "****"
	case length <= 8:
		return value[:2] + strings.Repeat("*", length-2)
	case length <= 16:
		return value[:3] + strings.Repeat("*", length-6) + value[length-3:]
	default:
		return value[:4] + "..." + strings.Repeat("*", 8) + "..." + value[length-4:]
	}
}


func createCipher(key string) (cipher.AEAD, error) {
	hash := sha256.Sum256([]byte(key))
	
	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return nil, err
	}
	
	return cipher.NewGCM(block)
}

func (m *CollectionMasker) encryptValue(value string) (string, error) {
	if m.cipher == nil {
		return "", fmt.Errorf("cipher not initialized")
	}
	
	nonce := make([]byte, m.cipher.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	
	ciphertext := m.cipher.Seal(nonce, nonce, []byte(value), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (m *CollectionMasker) DecryptValue(encrypted string) (string, error) {
	if m.cipher == nil {
		return "", fmt.Errorf("cipher not initialized")
	}
	
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	
	nonceSize := m.cipher.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := m.cipher.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}


func generateMaskingID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%d", b, time.Now().Unix())
}




func MaskCollection(collection *PostmanCollectionStructure) (*MaskingResult, error) {
	masker, err := NewCollectionMasker(nil)
	if err != nil {
		return nil, err
	}
	return masker.MaskCollection(collection)
}


func MaskCollectionWithKey(collection *PostmanCollectionStructure, encryptionKey string) (*MaskingResult, error) {
	config := DefaultMaskingConfig()
	config.PreserveMaskedValues = true
	config.MaskingKey = encryptionKey
	
	masker, err := NewCollectionMasker(config)
	if err != nil {
		return nil, err
	}
	return masker.MaskCollection(collection)
}
