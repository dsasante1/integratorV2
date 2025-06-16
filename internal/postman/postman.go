package postman

import (
	"encoding/json"
	"fmt"
	"net/http"
	"crypto/md5"
	"log/slog"
	"strings"
)

const (
	PostmanAPIBaseURL = "https://api.getpostman.com"
)

type PostmanCollection struct {
	ID   string `json:"id" validate:"required"`
	Name string `json:"name" validate:"required"`
}

type PostmanCollectionsResponse struct {
	Collections []PostmanCollection `json:"collections" validate:"required,dive"`
}

type PostmanCollectionResponse struct {
	Collection struct {
		ID   string          `json:"id" validate:"required"`
		Name string          `json:"name" validate:"required"`
		Info json.RawMessage `json:"info" validate:"required"`
		Item json.RawMessage `json:"item" validate:"required"`
	} `json:"collection" validate:"required"`
}

type Change struct {
	Type     string
	Path     string
	Modification *string
}

func GetCollections(apiKey string) ([]PostmanCollection, error) {
	req, err := http.NewRequest("GET", PostmanAPIBaseURL+"/collections", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("postman API returned status: %d", resp.StatusCode)
	}

	var result PostmanCollectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	return result.Collections, nil
}

func GetCollection(apiKey, collectionID string) (*PostmanCollectionStructure, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/collections/%s", PostmanAPIBaseURL, collectionID), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("postman API returned status: %d", resp.StatusCode)
	}

	var wrapper struct {
		Collection PostmanCollectionStructure `json:"collection"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	return &wrapper.Collection, nil
}

type CompareOptions struct {
	MaxDepth        int      // Maximum depth to traverse (0 = unlimited)
	MaxChanges      int      // Maximum number of changes to record (0 = unlimited)
	IgnorePaths     []string // Paths to ignore (e.g., ["info.version", "info._postman_id"])
	CompactChanges  bool     // If true, store minimal change info
	HashThreshold   int      // Size threshold for hashing large values (default 1000)
}


func DefaultPostmanOptions() *CompareOptions {
	return &CompareOptions{
		MaxDepth:      0, // Unlimited depth
		MaxChanges:    1000, // Limit changes to prevent memory issues
		HashThreshold: 1000, // Hash values larger than 1KB
		IgnorePaths: []string{
			"info._postman_id",     // Auto-generated IDs
			"info.version",         // Version info that might auto-update
			"**.id",                // Request IDs
			"**.currentHelper",     // UI state
			"**.helperAttributes",  // UI state
		},
		CompactChanges: true,
	}
}


func compareSnapshots(old, new json.RawMessage) []Change {
	opts := DefaultPostmanOptions()
	changes, err := ComparePostmanSnapshots(old, new, opts)
	if err != nil {
		slog.Error("Failed to compare snapshots", "error", err)
		return []Change{}
	}
	return changes
}

func ComparePostmanSnapshots(old, new json.RawMessage, opts *CompareOptions) ([]Change, error) {
	if opts == nil {
		opts = DefaultPostmanOptions()
	}

	var oldData, newData interface{}
	if err := json.Unmarshal(old, &oldData); err != nil {
		return nil, fmt.Errorf("unmarshal old snapshot: %w", err)
	}
	if err := json.Unmarshal(new, &newData); err != nil {
		return nil, fmt.Errorf("unmarshal new snapshot: %w", err)
	}

		slog.Info("alright we dey here ---->>>>", "old data", oldData)
	slog.Info("ok here is the old content ---->>", "new data", newData)

	ctx := &compareContext{
		changes:    make([]Change, 0),
		opts:       opts,
		pathIndex:  make(map[string]bool),
		changeCount: 0,
	}

	compareRecursive(ctx, "", oldData, newData, 0)
	return ctx.changes, nil
}

type compareContext struct {
	changes     []Change
	opts        *CompareOptions
	pathIndex   map[string]bool
	changeCount int
}

func compareRecursive(ctx *compareContext, path string, old, new interface{}, depth int) {

	if ctx.opts.MaxChanges > 0 && ctx.changeCount >= ctx.opts.MaxChanges {
		return
	}

	if ctx.opts.MaxDepth > 0 && depth > ctx.opts.MaxDepth {
		if !deepEqual(old, new) {
			addChange(ctx, "modified", path, new)
		}
		return
	}

	if shouldIgnorePath(path, ctx.opts.IgnorePaths) {
		return
	}

	if old == nil && new == nil {
		return
	}
	if old == nil {
		addChange(ctx, "added", path, new)
		return
	}
	if new == nil {
		addChange(ctx, "deleted", path, old)
		return
	}

	oldType := getJSONType(old)
	newType := getJSONType(new)
	if oldType != newType {
		addChange(ctx, "modified", path, new)
		return
	}

	
	switch oldVal := old.(type) {
	case map[string]interface{}:
		compareObjects(ctx, path, oldVal, new.(map[string]interface{}), depth)
	case []interface{}:
		compareArrays(ctx, path, oldVal, new.([]interface{}), depth)
	default:
		
		if !deepEqual(old, new) {
			addChange(ctx, "modified", path, new)
		}
	}
}

func compareObjects(ctx *compareContext, path string, old, new map[string]interface{}, depth int) {
	
	
	if isLargeObject(old) || isLargeObject(new) {
		if !deepEqual(old, new) {
			
			addChange(ctx, "modified", path, new)
		}
		return
	}

	
	for k, newVal := range new {
		newPath := joinPath(path, k)
		if oldVal, exists := old[k]; exists {
			compareRecursive(ctx, newPath, oldVal, newVal, depth+1)
		} else {
			addChange(ctx, "added", newPath, newVal)
		}
	}

	
	for k, oldVal := range old {
		if _, exists := new[k]; !exists {
			addChange(ctx, "deleted", joinPath(path, k), oldVal)
		}
	}
}

func compareArrays(ctx *compareContext, path string, old, new []interface{}, depth int) {
	
	
	
	
	if isPostmanItemArray(path) {
		comparePostmanItems(ctx, path, old, new, depth)
		return
	}

	
	maxLen := len(old)
	if len(new) > maxLen {
		maxLen = len(new)
	}

	for i := 0; i < maxLen; i++ {
		if ctx.opts.MaxChanges > 0 && ctx.changeCount >= ctx.opts.MaxChanges {
			return
		}

		indexPath := fmt.Sprintf("%s[%d]", path, i)
		if i < len(old) && i < len(new) {
			compareRecursive(ctx, indexPath, old[i], new[i], depth+1)
		} else if i >= len(new) {
			addChange(ctx, "deleted", indexPath, old[i])
		} else {
			addChange(ctx, "added", indexPath, new[i])
		}
	}
}

func comparePostmanItems(ctx *compareContext, path string, old, new []interface{}, depth int) {
	
	oldMap := make(map[string]interface{})
	oldIndices := make(map[string]int)
	
	
	for i, item := range old {
		if m, ok := item.(map[string]interface{}); ok {
			key := getItemKey(m)
			if key != "" {
				oldMap[key] = item
				oldIndices[key] = i
			}
		}
	}

	
	matched := make(map[string]bool)

	
	for i, newItem := range new {
		if m, ok := newItem.(map[string]interface{}); ok {
			key := getItemKey(m)
			indexPath := fmt.Sprintf("%s[%d]", path, i)
			
			if key != "" && oldMap[key] != nil {
				
				matched[key] = true
				oldIndex := oldIndices[key]
				if oldIndex != i {
					
					slog.Debug("Item moved", "path", path, "key", key, "from", oldIndex, "to", i)
				}
				compareRecursive(ctx, indexPath, oldMap[key], newItem, depth+1)
			} else {
				
				addChange(ctx, "added", indexPath, newItem)
			}
		}
	}

	
	for key, oldItem := range oldMap {
		if !matched[key] {
			indexPath := fmt.Sprintf("%s[%d]", path, oldIndices[key])
			addChange(ctx, "deleted", indexPath, oldItem)
		}
	}
}

func getItemKey(item map[string]interface{}) string {
	
	
	if id, ok := item["id"].(string); ok && id != "" {
		return "id:" + id
	}
	if name, ok := item["name"].(string); ok && name != "" {
		return "name:" + name
	}
	return ""
}

func addChange(ctx *compareContext, changeType, path string, value interface{}) {
	
	if ctx.pathIndex[path] {
		return
	}

	ctx.changeCount++
	ctx.pathIndex[path] = true

	var modification *string
	
	
	if value != nil {
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			slog.Warn("Failed to marshal change value", "path", path, "error", err)
			str := fmt.Sprintf("<<error: %v>>", err)
			modification = &str
		} else {
			
			if ctx.opts.HashThreshold > 0 && len(jsonBytes) > ctx.opts.HashThreshold {
				hash := fmt.Sprintf("<<hash:%x>>", md5.Sum(jsonBytes))
				modification = &hash
				
				
				
			} else {
				str := string(jsonBytes)
				modification = &str
			}
		}
	}

	change := Change{
		Type:         changeType,
		Path:         path,
		Modification: modification,
	}

	ctx.changes = append(ctx.changes, change)
}



func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	
	if strings.HasPrefix(key, "[") {
		return base + key
	}
	
	if needsQuoting(key) {
		return fmt.Sprintf("%s[\"%s\"]", base, key)
	}
	return base + "." + key
}

func needsQuoting(key string) bool {
	
	return strings.ContainsAny(key, ".[]()*?+\\^$|")
}

func shouldIgnorePath(path string, ignorePaths []string) bool {
	for _, ignore := range ignorePaths {
		if matchPath(path, ignore) {
			return true
		}
	}
	return false
}

func matchPath(path, pattern string) bool {
	
	
	
	if pattern == path {
		return true
	}
	if strings.Contains(pattern, "**") {
		prefix := strings.Split(pattern, "**")[0]
		return strings.HasPrefix(path, prefix)
	}
	
	return false
}

func isPostmanItemArray(path string) bool {
	
	segments := strings.Split(path, ".")
	for _, seg := range segments {
		if seg == "item" || seg == "request" || seg == "response" {
			return true
		}
	}
	return false
}

func isLargeObject(obj map[string]interface{}) bool {
	
	return len(obj) > 100
}

func getJSONType(v interface{}) string {
	switch v.(type) {
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

func deepEqual(a, b interface{}) bool {
	
	aType := getJSONType(a)
	bType := getJSONType(b)
	
	if aType != bType {
		return false
	}

	switch aVal := a.(type) {
	case map[string]interface{}:
		bMap := b.(map[string]interface{})
		if len(aVal) != len(bMap) {
			return false
		}
		for k, v := range aVal {
			if bv, ok := bMap[k]; !ok || !deepEqual(v, bv) {
				return false
			}
		}
		return true
		
	case []interface{}:
		bArr := b.([]interface{})
		if len(aVal) != len(bArr) {
			return false
		}
		for i := range aVal {
			if !deepEqual(aVal[i], bArr[i]) {
				return false
			}
		}
		return true
		
	default:
		
		return a == b
	}
}


func jsonEqual(a, b interface{}) bool {
	return deepEqual(a, b)
}
