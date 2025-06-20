package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"log/slog"
)

 
type ChangeDetail struct {
	ID             int64      `json:"id"`
	CollectionID   string     `json:"collection_id"`
	OldSnapshotID  *int64     `json:"old_snapshot_id"`
	NewSnapshotID  int64      `json:"new_snapshot_id"`
	ChangeType     string     `json:"change_type"`
	Path           string     `json:"path"`
	Modification   *string    `json:"modification"`
	CreatedAt      time.Time  `json:"created_at"`
	
	 
	HumanPath      string     `json:"human_path"`
	PathSegments   []string   `json:"path_segments"`
	EndpointName   string     `json:"endpoint_name,omitempty"`
	ResourceType   string     `json:"resource_type,omitempty"`
}

 
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

 
type ChangeNode struct {
	Name         string                 `json:"name"`
	Path         string                 `json:"path"`
	Type         string                 `json:"type"` // "folder" or "change"
	ChangeType   string                 `json:"change_type,omitempty"`
	ChangeCount  int                    `json:"change_count"`
	Children     []*ChangeNode          `json:"children,omitempty"`
	Change       *ChangeDetail          `json:"change,omitempty"`
}




// DiffResponse represents the complete diff between two snapshots
type DiffResponse struct {
	OldSnapshotID int64        `json:"old_snapshot_id"`
	NewSnapshotID int64        `json:"new_snapshot_id"`
	CollectionID  string       `json:"collection_id"`
	Changes       []DiffDetail `json:"changes"`
	Summary       DiffSummary  `json:"summary"`
}

// DiffDetail represents a single change with extracted values
type DiffDetail struct {
	ChangeDetail
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

// DiffSummary provides high-level statistics about the diff
type DiffSummary struct {
	TotalChanges     int            `json:"total_changes"`
	ChangesByType    map[string]int `json:"changes_by_type"`
	AffectedEndpoints []string      `json:"affected_endpoints"`
}

// Snapshot represents a stored snapshot
// type Snapshot struct {
// 	ID           int64            `json:"id"`
// 	CollectionID string           `json:"collection_id"`
// 	SnapshotTime time.Time        `json:"snapshot_time"`
// 	Content      json.RawMessage  `json:"content"`
// 	Hash         string           `json:"hash"`
// 	SnapshotID   *int64           `json:"snapshot_id"`
// }


func GetCollectionChangeSummary(collectionID string) (*ChangeSummary, error) {
	summary := &ChangeSummary{
		CollectionID:  collectionID,
		ChangesByType: make(map[string]int),
		ChangesByPath: make(map[string]int),
	}

	query := `
		SELECT 
			COUNT(*) as total,
			change_type,
			COUNT(*) as type_count
		FROM changes
		WHERE collection_id = $1
		GROUP BY change_type
	`
	
	rows, err := DB.Query(query, collectionID,)
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

	 
	timeQuery := `
		SELECT 
			MIN(created_at) as earliest,
			MAX(created_at) as latest
		FROM changes
		WHERE collection_id = $1
	`
	
	var earliest, latest sql.NullTime
	err = DB.QueryRow(timeQuery, collectionID).Scan(&earliest, &latest)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get time range: %w", err)
	}
	
	if earliest.Valid {
		summary.TimeRange.Earliest = earliest.Time
	}
	if latest.Valid {
		summary.TimeRange.Latest = latest.Time
	}

	 
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
			AND path LIKE 'collection.item[%]%'
	`
	
	endpointRows, err := DB.Query(endpointQuery, collectionID)
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

func GetChangeSummary(collectionID string, oldSnapshotID *int64, newSnapshotID *int64) (*ChangeSummary, error) {
	summary := &ChangeSummary{
		CollectionID:  collectionID,
		ChangesByType: make(map[string]int),
		ChangesByPath: make(map[string]int),
	}

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

func GetChanges(filter ChangeFilter) ([]*ChangeDetail, int, error) {
	var conditions []string
	var args []interface{}
	argCount := 0

	 
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

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM changes WHERE %s", whereClause)
	var totalCount int
	err := DB.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get count: %w", err)
	}

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

		 
		enhanceChangeDetail(change)
		changes = append(changes, change)
	}

	return changes, totalCount, nil
}

func GetChangeHierarchy(collectionID string, snapshotID int64) (*ChangeNode, error) {
	 
	filter := ChangeFilter{
		CollectionID: collectionID,
		SnapshotID:   &snapshotID,
		Limit:        10000, // Get all changes for hierarchy
	}
	
	changes, _, err := GetChanges(filter)
	if err != nil {
		return nil, err
	}

	 
	root := &ChangeNode{
		Name:     "Collection",
		Path:     "collection",
		Type:     "folder",
		Children: make([]*ChangeNode, 0),
	}

	 
	pathMap := make(map[string]*ChangeNode)
	pathMap["collection"] = root

	for _, change := range changes {
		addChangeToHierarchy(change, root, pathMap)
	}

	 
	calculateChangeCounts(root)

	return root, nil
}

 
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
		
		 
		endpoint := change.EndpointName
		if endpoint == "" {
			endpoint = "Collection Settings"
		}
		
		endpointChanges[endpoint] = append(endpointChanges[endpoint], change)
	}

	return endpointChanges, nil
}

func GetChangeDetails(changeID int64) (change ChangeDetail, err error) {
		 
	query := `
		SELECT 
			id, collection_id, old_snapshot_id, new_snapshot_id,
			change_type, path, modification, created_at
		FROM changes
		WHERE id = $1
	`
	change = ChangeDetail{}
	err = DB.QueryRow(query, changeID).Scan(
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
		if err == sql.ErrNoRows {
			return ChangeDetail{}, fmt.Errorf("Change not found")
		}

		return ChangeDetail{}, fmt.Errorf("internal server error")
	}

	EnhanceChangeDetail(&change)

	return change, nil
}

//TODO move to utilities Helper functions

func enhanceChangeDetail(change *ChangeDetail) {
	 
	change.PathSegments = parsePathSegments(change.Path)
	
	 
	change.HumanPath = generateHumanPath(change.PathSegments)
	
	 
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
	 
	if strings.Contains(path, "item[") {
		 
		re := regexp.MustCompile(`item\[(\d+)\]`)
		matches := re.FindStringSubmatch(path)
		if len(matches) > 1 {
			 
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
			 
			newNode := &ChangeNode{
				Name:     segment,
				Path:     currentPath,
				Type:     "folder",
				Children: make([]*ChangeNode, 0),
			}
			
			 
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


func GetSnapshotDiff(collectionID string, snapshotID int64) (DiffResponse, error) {

	var oldSnapshotID sql.NullInt64
	
	err := DB.Get(&oldSnapshotID, `
		SELECT old_snapshot_id
		FROM changes
		WHERE new_snapshot_id = $1 AND collection_id = $2
		LIMIT 1`,
		snapshotID, collectionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return DiffResponse{}, fmt.Errorf("no changes found for snapshot %d", snapshotID)
		}
		slog.Error("error fetching old snapshot ID", "error", err)
		return DiffResponse{}, fmt.Errorf("failed to fetch snapshot changes: %w", err)
	}

	if !oldSnapshotID.Valid {
		return DiffResponse{
			OldSnapshotID: 0,
			NewSnapshotID: snapshotID,
			CollectionID:  collectionID,
			Changes:       []DiffDetail{},
			Summary: DiffSummary{
				TotalChanges:      0,
				ChangesByType:     map[string]int{},
				AffectedEndpoints: []string{},
			},
		}, nil
	}


	changes, err := getChangesBetweenSnapshots(oldSnapshotID.Int64, snapshotID, collectionID)
	if err != nil {
		return DiffResponse{}, fmt.Errorf("failed to get changes: %w", err)
	}

	oldSnapshot, err := getSnapshot(oldSnapshotID.Int64)
	if err != nil {
		return DiffResponse{}, fmt.Errorf("failed to get old snapshot: %w", err)
	}

	newSnapshot, err := getSnapshot(snapshotID)
	if err != nil {
		return DiffResponse{}, fmt.Errorf("failed to get new snapshot: %w", err)
	}

	diffDetails := make([]DiffDetail, 0, len(changes))
	changesByType := make(map[string]int)
	endpointSet := make(map[string]bool)

	for _, change := range changes {

		enhanceChangeDetail(&change)

		oldValue, err := extractValueByPath(oldSnapshot.Content, change.Path, change.ChangeType == "added")
		if err != nil && change.ChangeType != "added" {
			slog.Warn("failed to extract old value", "path", change.Path, "error", err)
		}

		newValue, err := extractValueByPath(newSnapshot.Content, change.Path, change.ChangeType == "deleted")
		if err != nil && change.ChangeType != "deleted" {
			slog.Warn("failed to extract new value", "path", change.Path, "error", err)
		}

		diffDetail := DiffDetail{
			ChangeDetail: change,
			OldValue:     oldValue,
			NewValue:     newValue,
		}

		diffDetails = append(diffDetails, diffDetail)

		changesByType[change.ChangeType]++
		if change.EndpointName != "" {
			endpointSet[change.EndpointName] = true
		}
	}

	affectedEndpoints := make([]string, 0, len(endpointSet))
	for endpoint := range endpointSet {
		affectedEndpoints = append(affectedEndpoints, endpoint)
	}

	return DiffResponse{
		OldSnapshotID: oldSnapshotID.Int64,
		NewSnapshotID: snapshotID,
		CollectionID:  collectionID,
		Changes:       diffDetails,
		Summary: DiffSummary{
			TotalChanges:      len(diffDetails),
			ChangesByType:     changesByType,
			AffectedEndpoints: affectedEndpoints,
		},
	}, nil
}

func getChangesBetweenSnapshots(oldSnapshotID, newSnapshotID int64, collectionID string) ([]ChangeDetail, error) {
	query := `
		SELECT 
			id, collection_id, old_snapshot_id, new_snapshot_id,
			change_type, path, modification, created_at
		FROM changes
		WHERE old_snapshot_id = $1 AND new_snapshot_id = $2 AND collection_id = $3
		ORDER BY id ASC`

	rows, err := DB.Query(query, oldSnapshotID, newSnapshotID, collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query changes: %w", err)
	}
	defer rows.Close()

	var changes []ChangeDetail
	for rows.Next() {
		var change ChangeDetail
		var oldSnapshotID sql.NullInt64

		err := rows.Scan(
			&change.ID,
			&change.CollectionID,
			&oldSnapshotID,
			&change.NewSnapshotID,
			&change.ChangeType,
			&change.Path,
			&change.Modification,
			&change.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan change row: %w", err)
		}

		if oldSnapshotID.Valid {
			change.OldSnapshotID = &oldSnapshotID.Int64
		}

		changes = append(changes, change)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating change rows: %w", err)
	}

	return changes, nil
}

func getSnapshot(snapshotID int64) (*Snapshot, error) {
	var snapshot Snapshot
	
	query := `
		SELECT id, collection_id, snapshot_time, content, hash, snapshot_id
		FROM snapshots
		WHERE id = $1`

	err := DB.Get(&snapshot, query, snapshotID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("snapshot %d not found", snapshotID)
		}
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	return &snapshot, nil
}

func extractValueByPath(data json.RawMessage, path string, skipIfMissing bool) (interface{}, error) {

	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Split path into segments
	segments := parsePathSegments(path)
	current := jsonData

	// Navigate through the JSON structure
	for i, segment := range segments {
		if current == nil {
			if skipIfMissing {
				return nil, nil
			}
			return nil, fmt.Errorf("null value encountered at segment %d (%s)", i, segment)
		}

		switch v := current.(type) {
		case map[string]interface{}:
			if strings.HasPrefix(segment, "[") && strings.HasSuffix(segment, "]") {
				// This is an array index, but we're in an object - path error
				if skipIfMissing {
					return nil, nil
				}
				return nil, fmt.Errorf("expected object key but got array index %s", segment)
			}
			
			val, exists := v[segment]
			if !exists {
				if skipIfMissing {
					return nil, nil
				}
				return nil, fmt.Errorf("key '%s' not found in object", segment)
			}
			current = val

		case []interface{}:
			if !strings.HasPrefix(segment, "[") || !strings.HasSuffix(segment, "]") {
				if skipIfMissing {
					return nil, nil
				}
				return nil, fmt.Errorf("expected array index but got object key %s", segment)
			}
			
			// Extract index from [n] format
			indexStr := segment[1 : len(segment)-1]
			index := 0
			if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
				if skipIfMissing {
					return nil, nil
				}
				return nil, fmt.Errorf("invalid array index %s", segment)
			}
			
			if index < 0 || index >= len(v) {
				if skipIfMissing {
					return nil, nil
				}
				return nil, fmt.Errorf("array index %d out of bounds (length: %d)", index, len(v))
			}
			current = v[index]

		default:
			// We've reached a primitive value but still have more segments
			if skipIfMissing {
				return nil, nil
			}
			return nil, fmt.Errorf("cannot navigate further into primitive value at segment %s", segment)
		}
	}

	return current, nil
}
