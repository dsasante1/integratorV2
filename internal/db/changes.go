package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"regexp"
)

// ChangeDetail represents a single change with full details
type ChangeDetail struct {
	ID             int64      `json:"id"`
	CollectionID   string     `json:"collection_id"`
	OldSnapshotID  *int64     `json:"old_snapshot_id"`
	NewSnapshotID  int64      `json:"new_snapshot_id"`
	ChangeType     string     `json:"change_type"`
	Path           string     `json:"path"`
	Modification   *string    `json:"modification"`
	CreatedAt      time.Time  `json:"created_at"`
	
	// Computed fields for display
	HumanPath      string     `json:"human_path"`
	PathSegments   []string   `json:"path_segments"`
	EndpointName   string     `json:"endpoint_name,omitempty"`
	ResourceType   string     `json:"resource_type,omitempty"`
}

// ChangeSummary provides high-level statistics
type ChangeSummary struct {
	CollectionID     string                 `json:"collection_id"`
	TotalChanges     int                    `json:"total_changes"`
	ChangesByType    map[string]int         `json:"changes_by_type"`
	AffectedEndpoints []string              `json:"affected_endpoints"`
	TimeRange        TimeRange              `json:"time_range"`
	ChangesByPath    map[string]int         `json:"changes_by_path"`
}

type TimeRange struct {
	Earliest time.Time `json:"earliest"`
	Latest   time.Time `json:"latest"`
}

// ChangeFilter provides filtering options
type ChangeFilter struct {
	CollectionID   string
	ChangeTypes    []string
	PathPattern    string
	StartTime      *time.Time
	EndTime        *time.Time
	SnapshotID     *int64
	Limit          int
	Offset         int
}

// ChangeNode represents a hierarchical view of changes
type ChangeNode struct {
	Name         string                 `json:"name"`
	Path         string                 `json:"path"`
	Type         string                 `json:"type"` // "folder" or "change"
	ChangeType   string                 `json:"change_type,omitempty"`
	ChangeCount  int                    `json:"change_count"`
	Children     []*ChangeNode          `json:"children,omitempty"`
	Change       *ChangeDetail          `json:"change,omitempty"`
}

// GetChangeSummary retrieves a high-level summary of changes
func GetChangeSummary(collectionID string, oldSnapshotID *int64, newSnapshotID int64) (*ChangeSummary, error) {
	summary := &ChangeSummary{
		CollectionID:  collectionID,
		ChangesByType: make(map[string]int),
		ChangesByPath: make(map[string]int),
	}

	// Get total changes and changes by type
	query := `
		SELECT 
			COUNT(*) as total,
			change_type,
			COUNT(*) as type_count
		FROM changes
		WHERE collection_id = $1
			AND ($2::INTEGER IS NULL OR old_snapshot_id = $2)
			AND new_snapshot_id = $3
		GROUP BY change_type
	`
	
	rows, err := DB.Query(query, collectionID, oldSnapshotID, newSnapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get change summary: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var total int
		var changeType string
		var typeCount int
		
		if err := rows.Scan(&total, &changeType, &typeCount); err != nil {
			return nil, err
		}
		
		summary.TotalChanges = total
		summary.ChangesByType[changeType] = typeCount
	}

	// Get time range
	timeQuery := `
		SELECT 
			MIN(created_at) as earliest,
			MAX(created_at) as latest
		FROM changes
		WHERE collection_id = $1
			AND ($2::INTEGER IS NULL OR old_snapshot_id = $2)
			AND new_snapshot_id = $3
	`
	
	var earliest, latest sql.NullTime
	err = DB.QueryRow(timeQuery, collectionID, oldSnapshotID, newSnapshotID).Scan(&earliest, &latest)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get time range: %w", err)
	}
	
	if earliest.Valid {
		summary.TimeRange.Earliest = earliest.Time
	}
	if latest.Valid {
		summary.TimeRange.Latest = latest.Time
	}

	// Get affected endpoints
	endpointQuery := `
		SELECT DISTINCT
			CASE 
				WHEN path LIKE 'collection.item[%].name' THEN modification
				WHEN path LIKE 'collection.item[%].request%' THEN 
					SUBSTRING(path FROM 'collection\.item\[(\d+)\]')
				ELSE NULL
			END as endpoint
		FROM changes
		WHERE collection_id = $1
			AND ($2::INTEGER IS NULL OR old_snapshot_id = $2)
			AND new_snapshot_id = $3
			AND path LIKE 'collection.item[%]%'
	`
	
	endpointRows, err := DB.Query(endpointQuery, collectionID, oldSnapshotID, newSnapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get affected endpoints: %w", err)
	}
	defer endpointRows.Close()

	endpointSet := make(map[string]bool)
	for endpointRows.Next() {
		var endpoint sql.NullString
		if err := endpointRows.Scan(&endpoint); err != nil {
			continue
		}
		if endpoint.Valid && endpoint.String != "" {
			endpointSet[endpoint.String] = true
		}
	}
	
	for endpoint := range endpointSet {
		summary.AffectedEndpoints = append(summary.AffectedEndpoints, endpoint)
	}

	return summary, nil
}

// GetChanges retrieves changes with filtering and pagination
func GetChanges(filter ChangeFilter) ([]*ChangeDetail, int, error) {
	var conditions []string
	var args []interface{}
	argCount := 0

	// Build WHERE conditions
	conditions = append(conditions, fmt.Sprintf("collection_id = $%d", argCount+1))
	args = append(args, filter.CollectionID)
	argCount++

	if filter.SnapshotID != nil {
		conditions = append(conditions, fmt.Sprintf("new_snapshot_id = $%d", argCount+1))
		args = append(args, *filter.SnapshotID)
		argCount++
	}

	if len(filter.ChangeTypes) > 0 {
		placeholders := make([]string, len(filter.ChangeTypes))
		for i, ct := range filter.ChangeTypes {
			placeholders[i] = fmt.Sprintf("$%d", argCount+1+i)
			args = append(args, ct)
		}
		conditions = append(conditions, fmt.Sprintf("change_type IN (%s)", strings.Join(placeholders, ",")))
		argCount += len(filter.ChangeTypes)
	}

	if filter.PathPattern != "" {
		conditions = append(conditions, fmt.Sprintf("path LIKE $%d", argCount+1))
		args = append(args, "%"+filter.PathPattern+"%")
		argCount++
	}

	if filter.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argCount+1))
		args = append(args, *filter.StartTime)
		argCount++
	}

	if filter.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argCount+1))
		args = append(args, *filter.EndTime)
		argCount++
	}

	whereClause := strings.Join(conditions, " AND ")

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM changes WHERE %s", whereClause)
	var totalCount int
	err := DB.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get count: %w", err)
	}

	// Get paginated results
	query := fmt.Sprintf(`
		SELECT 
			id, collection_id, old_snapshot_id, new_snapshot_id,
			change_type, path, modification, created_at
		FROM changes
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argCount+1, argCount+2)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query changes: %w", err)
	}
	defer rows.Close()

	var changes []*ChangeDetail
	for rows.Next() {
		change := &ChangeDetail{}
		err := rows.Scan(
			&change.ID,
			&change.CollectionID,
			&change.OldSnapshotID,
			&change.NewSnapshotID,
			&change.ChangeType,
			&change.Path,
			&change.Modification,
			&change.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan change: %w", err)
		}

		// Enhance with computed fields
		enhanceChangeDetail(change)
		changes = append(changes, change)
	}

	return changes, totalCount, nil
}

// GetChangeHierarchy retrieves changes organized in a tree structure
func GetChangeHierarchy(collectionID string, snapshotID int64) (*ChangeNode, error) {
	// First, get all changes
	filter := ChangeFilter{
		CollectionID: collectionID,
		SnapshotID:   &snapshotID,
		Limit:        10000, // Get all changes for hierarchy
	}
	
	changes, _, err := GetChanges(filter)
	if err != nil {
		return nil, err
	}

	// Build tree structure
	root := &ChangeNode{
		Name:     "Collection",
		Path:     "collection",
		Type:     "folder",
		Children: make([]*ChangeNode, 0),
	}

	// Group changes by path segments
	pathMap := make(map[string]*ChangeNode)
	pathMap["collection"] = root

	for _, change := range changes {
		addChangeToHierarchy(change, root, pathMap)
	}

	// Calculate change counts
	calculateChangeCounts(root)

	return root, nil
}

// GetChangesByEndpoint retrieves changes grouped by endpoint
func GetChangesByEndpoint(collectionID string, snapshotID int64) (map[string][]*ChangeDetail, error) {
	query := `
		SELECT 
			id, collection_id, old_snapshot_id, new_snapshot_id,
			change_type, path, modification, created_at
		FROM changes
		WHERE collection_id = $1 AND new_snapshot_id = $2
		ORDER BY path
	`

	rows, err := DB.Query(query, collectionID, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to query changes: %w", err)
	}
	defer rows.Close()

	endpointChanges := make(map[string][]*ChangeDetail)
	
	for rows.Next() {
		change := &ChangeDetail{}
		err := rows.Scan(
			&change.ID,
			&change.CollectionID,
			&change.OldSnapshotID,
			&change.NewSnapshotID,
			&change.ChangeType,
			&change.Path,
			&change.Modification,
			&change.CreatedAt,
		)
		if err != nil {
			continue
		}

		enhanceChangeDetail(change)
		
		// Group by endpoint name
		endpoint := change.EndpointName
		if endpoint == "" {
			endpoint = "Collection Settings"
		}
		
		endpointChanges[endpoint] = append(endpointChanges[endpoint], change)
	}

	return endpointChanges, nil
}

//TODO move these to utilities

func enhanceChangeDetail(change *ChangeDetail) {
	// Parse path into segments
	change.PathSegments = parsePathSegments(change.Path)
	
	// Generate human-readable path
	change.HumanPath = generateHumanPath(change.PathSegments)
	
	// Extract endpoint name and resource type
	change.EndpointName = extractEndpointName(change.Path, change.Modification)
	change.ResourceType = extractResourceType(change.Path)
}

func parsePathSegments(path string) []string {
	var segments []string
	current := ""
	inBracket := false
	
	for _, ch := range path {
		switch ch {
		case '[':
			if current != "" {
				segments = append(segments, current)
				current = ""
			}
			inBracket = true
			current = "["
		case ']':
			current += "]"
			segments = append(segments, current)
			current = ""
			inBracket = false
		case '.':
			if !inBracket && current != "" {
				segments = append(segments, current)
				current = ""
			} else {
				current += string(ch)
			}
		default:
			current += string(ch)
		}
	}
	
	if current != "" {
		segments = append(segments, current)
	}
	
	return segments
}

func generateHumanPath(segments []string) string {
	var parts []string
	
	for i, seg := range segments {
		switch seg {
		case "collection":
			parts = append(parts, "Collection")
		case "info":
			parts = append(parts, "Info")
		case "item":
			parts = append(parts, "Items")
		case "request":
			parts = append(parts, "Request")
		case "response":
			parts = append(parts, "Response")
		case "body":
			parts = append(parts, "Body")
		case "header":
			parts = append(parts, "Headers")
		case "url":
			parts = append(parts, "URL")
		default:
			if strings.HasPrefix(seg, "[") && strings.HasSuffix(seg, "]") {
				// Array index
				index := seg[1:len(seg)-1]
				if i > 0 && segments[i-1] == "item" {
					parts = append(parts, fmt.Sprintf("#%s", index))
				} else {
					parts = append(parts, seg)
				}
			} else {
				parts = append(parts, seg)
			}
		}
	}
	
	return strings.Join(parts, " → ")
}

func extractEndpointName(path string, modification *string) string {
	// Try to extract from path
	if strings.Contains(path, "item[") {
		// Extract item index
		re := regexp.MustCompile(`item\[(\d+)\]`)
		matches := re.FindStringSubmatch(path)
		if len(matches) > 1 {
			// Try to get name from modification if it's a name change
			if strings.HasSuffix(path, "].name") && modification != nil {
				var name string
				if err := json.Unmarshal([]byte(*modification), &name); err == nil {
					return name
				}
			}
			return fmt.Sprintf("Endpoint %s", matches[1])
		}
	}
	return ""
}

func extractResourceType(path string) string {
	if strings.Contains(path, ".request") {
		return "request"
	} else if strings.Contains(path, ".response") {
		return "response"
	} else if strings.Contains(path, ".info") {
		return "info"
	} else if strings.Contains(path, ".item") {
		return "endpoint"
	}
	return "collection"
}

func addChangeToHierarchy(change *ChangeDetail, root *ChangeNode, pathMap map[string]*ChangeNode) {
	segments := change.PathSegments
	currentNode := root
	currentPath := "collection"
	
	for i, segment := range segments {
		if i == 0 && segment == "collection" {
			continue
		}
		
		currentPath += "." + segment
		
		if node, exists := pathMap[currentPath]; exists {
			currentNode = node
		} else {
			// Create new node
			newNode := &ChangeNode{
				Name:     segment,
				Path:     currentPath,
				Type:     "folder",
				Children: make([]*ChangeNode, 0),
			}
			
			// Make it more readable
			if strings.HasPrefix(segment, "[") && strings.HasSuffix(segment, "]") {
				index := segment[1:len(segment)-1]
				if i > 0 && segments[i-1] == "item" {
					newNode.Name = fmt.Sprintf("Endpoint %s", index)
					if change.EndpointName != "" {
						newNode.Name = change.EndpointName
					}
				}
			}
			
			currentNode.Children = append(currentNode.Children, newNode)
			pathMap[currentPath] = newNode
			currentNode = newNode
		}
	}
	
	// Add the actual change as a leaf node
	changeNode := &ChangeNode{
		Name:       fmt.Sprintf("%s: %s", change.ChangeType, segments[len(segments)-1]),
		Path:       change.Path,
		Type:       "change",
		ChangeType: change.ChangeType,
		Change:     change,
	}
	
	currentNode.Children = append(currentNode.Children, changeNode)
}

func calculateChangeCounts(node *ChangeNode) int {
	if node.Type == "change" {
		return 1
	}
	
	count := 0
	for _, child := range node.Children {
		count += calculateChangeCounts(child)
	}
	
	node.ChangeCount = count
	return count
}