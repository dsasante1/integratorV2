package postman

import (
	"encoding/json"
	"fmt"
	"net/http"
	"crypto/md5"
	"log/slog"
	"strings"
	"regexp"
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

type CompareOptions struct {
	MaxDepth        int      // Maximum depth to traverse (0 = unlimited)
	MaxChanges      int      // Maximum number of changes to record (0 = unlimited)
	IgnorePaths     []string // Paths to ignore (e.g., ["info.version", "info._postman_id"])
	CompactChanges  bool     // If true, store minimal change info
	HashThreshold   int      // Size threshold for hashing large values (default 1000)
}


type compareContext struct {
	changes     []Change
	opts        *CompareOptions
	pathIndex   map[string]bool
	changeCount int
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



func DefaultPostmanOptions() *CompareOptions {
	return &CompareOptions{
		MaxDepth:      0, // Unlimited depth
		MaxChanges:    1000, // Limit changes to prevent memory issues
		HashThreshold: 1000, // Hash values larger than 1KB
		IgnorePaths: []string{
			"collection.info._postman_id",
			"collection.info.version",
			"collection.item[*].id",  // More specific paths
			"**.currentHelper",
			"**.helperAttributes",
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

	var oldData, newData any

	if err := json.Unmarshal(old, &oldData); err != nil {
		return nil, fmt.Errorf("unmarshal old snapshot: %w", err)
	}
	if err := json.Unmarshal(new, &newData); err != nil {
		return nil, fmt.Errorf("unmarshal new snapshot: %w", err)
	}

	ctx := &compareContext{
		changes:     make([]Change, 0),
		opts:        opts,
		pathIndex:   make(map[string]bool),
		changeCount: 0,
	}

	// First check for structural changes at ANY level
	if hasStructuralChanges(ctx, "", oldData, newData) {
		// If structural changes exist, process ONLY structural changes
		processStructuralChanges(ctx, "", oldData, newData, 0)
		return ctx.changes, nil
	}

	// No structural changes anywhere, proceed with content comparison
	compareRecursive(ctx, "", oldData, newData, 0)
	return ctx.changes, nil
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
    // Use a combination of fields for better uniqueness
    if request, ok := item["request"].(map[string]interface{}); ok {
        if url, ok := request["url"].(map[string]interface{}); ok {
            if raw, ok := url["raw"].(string); ok {
                if name, ok := item["name"].(string); ok {
                    return fmt.Sprintf("%s:%s", name, raw)
                }
            }
        }
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
        parts := strings.Split(pattern, "**")
        if len(parts) == 2 {
            prefix := parts[0]
            suffix := parts[1]
            return strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix)
        }
    }
    
    pattern = strings.ReplaceAll(pattern, "[*]", `\[\d+\]`)
    matched, _ := regexp.MatchString("^"+pattern+"$", path)
    return matched
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




// StructuralChange represents a structural difference between JSONs
type StructuralChange struct {
	Type string // "added_field" or "deleted_field"
	Path string
	Field string // The field name that was added/deleted
}

// CompareStructure performs a shallow structural comparison to detect field additions/deletions
func CompareStructure(ctx *compareContext, path string, old, new interface{}) []StructuralChange {
	var structural []StructuralChange
	
	// Only compare structure for objects
	oldObj, oldIsObj := old.(map[string]interface{})
	newObj, newIsObj := new.(map[string]interface{})
	
	if !oldIsObj || !newIsObj {
		return structural
	}
	
	// Check for added fields (in new but not in old)
	for key := range newObj {
		if _, exists := oldObj[key]; !exists {
			structural = append(structural, StructuralChange{
				Type:  "added_field",
				Path:  path,
				Field: key,
			})
		}
	}
	
	// Check for deleted fields (in old but not in new)
	for key := range oldObj {
		if _, exists := newObj[key]; !exists {
			structural = append(structural, StructuralChange{
				Type:  "deleted_field",
				Path:  path,
				Field: key,
			})
		}
	}
	
	return structural
}

// ComparePostmanSnapshotsWithStructure performs two-phase comparison
func ComparePostmanSnapshotsWithStructure(old, new json.RawMessage, opts *CompareOptions) ([]Change, []StructuralChange, error) {
	if opts == nil {
		opts = DefaultPostmanOptions()
	}

	var oldData, newData any

	if err := json.Unmarshal(old, &oldData); err != nil {
		return nil, nil, fmt.Errorf("unmarshal old snapshot: %w", err)
	}
	if err := json.Unmarshal(new, &newData); err != nil {
		return nil, nil, fmt.Errorf("unmarshal new snapshot: %w", err)
	}

	ctx := &compareContext{
		changes:     make([]Change, 0),
		opts:        opts,
		pathIndex:   make(map[string]bool),
		changeCount: 0,
	}

	// Phase 1: Structural comparison
	structural := compareStructureRecursive(ctx, "", oldData, newData, 0)
	
	// Phase 2: Content comparison (your existing logic)
	compareRecursive(ctx, "", oldData, newData, 0)
	
	return ctx.changes, structural, nil
}

// compareStructureRecursive recursively compares structure throughout the JSON tree
func compareStructureRecursive(ctx *compareContext, path string, old, new interface{}, depth int) []StructuralChange {
	var allStructural []StructuralChange
	
	if shouldIgnorePath(path, ctx.opts.IgnorePaths) {
		return allStructural
	}
	
	// Get structural changes at this level
	structural := CompareStructure(ctx, path, old, new)
	allStructural = append(allStructural, structural...)
	
	// Recursively check nested objects
	oldObj, oldIsObj := old.(map[string]interface{})
	newObj, newIsObj := new.(map[string]interface{})
	
	if oldIsObj && newIsObj {
		// For fields that exist in both, recursively check their structure
		for key := range oldObj {
			if newVal, exists := newObj[key]; exists {
				newPath := joinPath(path, key)
				nested := compareStructureRecursive(ctx, newPath, oldObj[key], newVal, depth+1)
				allStructural = append(allStructural, nested...)
			}
		}
	}
	
	// Handle arrays
	oldArr, oldIsArr := old.([]interface{})
	newArr, newIsArr := new.([]interface{})
	
	if oldIsArr && newIsArr {
		maxLen := len(oldArr)
		if len(newArr) > maxLen {
			maxLen = len(newArr)
		}
		
		for i := 0; i < maxLen && i < len(oldArr) && i < len(newArr); i++ {
			indexPath := fmt.Sprintf("%s[%d]", path, i)
			nested := compareStructureRecursive(ctx, indexPath, oldArr[i], newArr[i], depth+1)
			allStructural = append(allStructural, nested...)
		}
	}
	
	return allStructural
}

// Enhanced compareRecursive that can skip structural differences if needed
func compareRecursiveEnhanced(ctx *compareContext, path string, old, new interface{}, depth int, skipStructural bool) {
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
		if !skipStructural {
			addChange(ctx, "added", path, new)
		}
		return
	}
	if new == nil {
		if !skipStructural {
			addChange(ctx, "deleted", path, old)
		}
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
		compareObjectsEnhanced(ctx, path, oldVal, new.(map[string]interface{}), depth, skipStructural)
	case []interface{}:
		compareArrays(ctx, path, oldVal, new.([]interface{}), depth)
	default:
		if !deepEqual(old, new) {
			addChange(ctx, "modified", path, new)
		}
	}
}

// Enhanced compareObjects that can handle structural skip
func compareObjectsEnhanced(ctx *compareContext, path string, old, new map[string]interface{}, depth int, skipStructural bool) {
	if isLargeObject(old) || isLargeObject(new) {
		if !deepEqual(old, new) {
			addChange(ctx, "modified", path, new)
		}
		return
	}

	// Check modifications for existing fields
	for k, newVal := range new {
		newPath := joinPath(path, k)
		if oldVal, exists := old[k]; exists {
			compareRecursiveEnhanced(ctx, newPath, oldVal, newVal, depth+1, skipStructural)
		} else if !skipStructural {
			addChange(ctx, "added", newPath, newVal)
		}
	}

	// Check deletions
	if !skipStructural {
		for k, oldVal := range old {
			if _, exists := new[k]; !exists {
				addChange(ctx, "deleted", joinPath(path, k), oldVal)
			}
		}
	}
}

// Example usage function
func AnalyzeJSONDifferences(old, new json.RawMessage) {
	opts := DefaultPostmanOptions()
	
	// Get both structural and content changes
	changes, structural, err := ComparePostmanSnapshotsWithStructure(old, new, opts)
	if err != nil {
		slog.Error("Failed to compare", "error", err)
		return
	}
	
	// Log structural changes
	if len(structural) > 0 {
		slog.Info("Structural differences found", "count", len(structural))
		for _, s := range structural {
			slog.Info("Structure change", 
				"type", s.Type,
				"path", s.Path,
				"field", s.Field)
		}
	}
	
	// Log content changes
	if len(changes) > 0 {
		slog.Info("Content changes found", "count", len(changes))
		for _, c := range changes {
			var value string
			if c.Modification != nil {
				value = *c.Modification
			}
			slog.Info("Content change",
				"type", c.Type,
				"path", c.Path,
				"value", value)
		}
	}
}

// Alternative: Get only structural differences without content comparison
func GetStructuralDifferencesOnly(old, new json.RawMessage, opts *CompareOptions) ([]StructuralChange, error) {
	if opts == nil {
		opts = DefaultPostmanOptions()
	}

	var oldData, newData any

	if err := json.Unmarshal(old, &oldData); err != nil {
		return nil, fmt.Errorf("unmarshal old snapshot: %w", err)
	}
	if err := json.Unmarshal(new, &newData); err != nil {
		return nil, fmt.Errorf("unmarshal new snapshot: %w", err)
	}

	ctx := &compareContext{
		changes:     make([]Change, 0),
		opts:        opts,
		pathIndex:   make(map[string]bool),
		changeCount: 0,
	}

	return compareStructureRecursive(ctx, "", oldData, newData, 0), nil
}

func hasStructuralChanges(ctx *compareContext, path string, old, new interface{}) bool {
	// Check if both are objects
	oldObj, oldIsObj := old.(map[string]interface{})
	newObj, newIsObj := new.(map[string]interface{})
	
	if oldIsObj && newIsObj {
		// Check for added fields (in new but not in old)
		for key := range newObj {
			if shouldIgnorePath(joinPath(path, key), ctx.opts.IgnorePaths) {
				continue
			}
			if _, exists := oldObj[key]; !exists {
				return true
			}
		}
		
		// Check for deleted fields (in old but not in new)
		for key := range oldObj {
			if shouldIgnorePath(joinPath(path, key), ctx.opts.IgnorePaths) {
				continue
			}
			if _, exists := newObj[key]; !exists {
				return true
			}
		}
		
		// Recursively check nested objects for structural changes
		for key := range oldObj {
			if newVal, exists := newObj[key]; exists {
				newPath := joinPath(path, key)
				if shouldIgnorePath(newPath, ctx.opts.IgnorePaths) {
					continue
				}
				if hasStructuralChanges(ctx, newPath, oldObj[key], newVal) {
					return true
				}
			}
		}
		return false
	}
	
	// Check arrays for length differences (additions/deletions)
	oldArr, oldIsArr := old.([]interface{})
	newArr, newIsArr := new.([]interface{})
	
	if oldIsArr && newIsArr {
		// For Postman item arrays, check by comparing items by key
		if isPostmanItemArray(path) {
			oldKeys := make(map[string]bool)
			newKeys := make(map[string]bool)
			
			for _, item := range oldArr {
				if m, ok := item.(map[string]interface{}); ok {
					if key := getItemKey(m); key != "" {
						oldKeys[key] = true
					}
				}
			}
			
			for _, item := range newArr {
				if m, ok := item.(map[string]interface{}); ok {
					if key := getItemKey(m); key != "" {
						newKeys[key] = true
					}
				}
			}
			
			// Check if any items were added or deleted
			for key := range oldKeys {
				if !newKeys[key] {
					return true // Item deleted
				}
			}
			for key := range newKeys {
				if !oldKeys[key] {
					return true // Item added
				}
			}
			
			// Recursively check remaining items
			for i := 0; i < len(oldArr) && i < len(newArr); i++ {
				if hasStructuralChanges(ctx, fmt.Sprintf("%s[%d]", path, i), oldArr[i], newArr[i]) {
					return true
				}
			}
		} else {
			// For regular arrays, length difference means structural change
			if len(oldArr) != len(newArr) {
				return true
			}
			// Recursively check each element
			for i := 0; i < len(oldArr); i++ {
				if hasStructuralChanges(ctx, fmt.Sprintf("%s[%d]", path, i), oldArr[i], newArr[i]) {
					return true
				}
			}
		}
	}
	
	return false
}

func processStructuralChanges(ctx *compareContext, path string, old, new interface{}, depth int) {
	if ctx.opts.MaxChanges > 0 && ctx.changeCount >= ctx.opts.MaxChanges {
		return
	}
	
	if ctx.opts.MaxDepth > 0 && depth > ctx.opts.MaxDepth {
		return
	}
	
	// Handle objects
	oldObj, oldIsObj := old.(map[string]interface{})
	newObj, newIsObj := new.(map[string]interface{})
	
	if oldIsObj && newIsObj {
		// Process added fields
		for key, newVal := range newObj {
			newPath := joinPath(path, key)
			if shouldIgnorePath(newPath, ctx.opts.IgnorePaths) {
				continue
			}
			if _, exists := oldObj[key]; !exists {
				// Field was added
				addChange(ctx, "added", newPath, newVal)
			}
		}
		
		// Process deleted fields
		for key, oldVal := range oldObj {
			newPath := joinPath(path, key)
			if shouldIgnorePath(newPath, ctx.opts.IgnorePaths) {
				continue
			}
			if _, exists := newObj[key]; !exists {
				// Field was deleted
				addChange(ctx, "deleted", newPath, oldVal)
			}
		}
		
		// Recursively check for structural changes in nested objects
		for key := range oldObj {
			if newVal, exists := newObj[key]; exists {
				newPath := joinPath(path, key)
				if shouldIgnorePath(newPath, ctx.opts.IgnorePaths) {
					continue
				}
				processStructuralChanges(ctx, newPath, oldObj[key], newVal, depth+1)
			}
		}
		return
	}
	
	oldArr, oldIsArr := old.([]interface{})
	newArr, newIsArr := new.([]interface{})
	
	if oldIsArr && newIsArr {
		if isPostmanItemArray(path) {
			// Special handling for Postman item arrays
			processPostmanItemStructuralChanges(ctx, path, oldArr, newArr, depth)
		} else {
			if len(oldArr) != len(newArr) {
				maxLen := len(oldArr)
				if len(newArr) > maxLen {
					maxLen = len(newArr)
				}
				
				for i := 0; i < maxLen; i++ {
					if i >= len(oldArr) {
						// Element added
						addChange(ctx, "added", fmt.Sprintf("%s[%d]", path, i), newArr[i])
					} else if i >= len(newArr) {
						// Element deleted
						addChange(ctx, "deleted", fmt.Sprintf("%s[%d]", path, i), oldArr[i])
					}
				}
			}
			
			minLen := len(oldArr)
			if len(newArr) < minLen {
				minLen = len(newArr)
			}
			for i := 0; i < minLen; i++ {
				processStructuralChanges(ctx, fmt.Sprintf("%s[%d]", path, i), oldArr[i], newArr[i], depth+1)
			}
		}
	}
}

// Special handling for Postman item arrays to avoid false modifications
func processPostmanItemStructuralChanges(ctx *compareContext, path string, oldArr, newArr []interface{}, depth int) {
	oldMap := make(map[string]interface{})
	oldIndices := make(map[string]int)
	
	// Build map of old items by key
	for i, item := range oldArr {
		if m, ok := item.(map[string]interface{}); ok {
			key := getItemKey(m)
			if key != "" {
				oldMap[key] = item
				oldIndices[key] = i
			}
		}
	}
	
	// Build map of new items by key
	newMap := make(map[string]interface{})
	newIndices := make(map[string]int)
	
	for i, item := range newArr {
		if m, ok := item.(map[string]interface{}); ok {
			key := getItemKey(m)
			if key != "" {
				newMap[key] = item
				newIndices[key] = i
			}
		}
	}
	
	// Report deletions
	for key, oldItem := range oldMap {
		if _, exists := newMap[key]; !exists {
			indexPath := fmt.Sprintf("%s[%d]", path, oldIndices[key])
			addChange(ctx, "deleted", indexPath, oldItem)
		}
	}
	
	// Report additions
	for key, newItem := range newMap {
		if _, exists := oldMap[key]; !exists {
			indexPath := fmt.Sprintf("%s[%d]", path, newIndices[key])
			addChange(ctx, "added", indexPath, newItem)
		}
	}
	
	for key, newItem := range newMap {
		if oldItem, exists := oldMap[key]; exists {
			indexPath := fmt.Sprintf("%s[%d]", path, newIndices[key])
			processStructuralChanges(ctx, indexPath, oldItem, newItem, depth+1)
		}
	}
}