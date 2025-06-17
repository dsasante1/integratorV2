package db

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ChangeImpactAnalysis represents the impact analysis of changes
type ChangeImpactAnalysis struct {
	CollectionID      string                `json:"collection_id"`
	SnapshotID        int64                 `json:"snapshot_id"`
	BreakingChanges   []*ImpactDetail       `json:"breaking_changes"`
	SecurityChanges   []*ImpactDetail       `json:"security_changes"`
	DataChanges       []*ImpactDetail       `json:"data_changes"`
	CosmeticChanges   []*ImpactDetail       `json:"cosmetic_changes"`
	Summary           ImpactSummary         `json:"summary"`
}

type ImpactDetail struct {
	Change      *ChangeDetail `json:"change"`
	Impact      string        `json:"impact"`
	Severity    string        `json:"severity"` // high, medium, low
	Suggestions []string      `json:"suggestions,omitempty"`
}

type ImpactSummary struct {
	TotalBreaking   int     `json:"total_breaking"`
	TotalSecurity   int     `json:"total_security"`
	TotalData       int     `json:"total_data"`
	TotalCosmetic   int     `json:"total_cosmetic"`
	RiskScore       float64 `json:"risk_score"` // 0-100
	Recommendation  string  `json:"recommendation"`
}

// ChangeFrequencyAnalysis tracks which paths change most frequently
type ChangeFrequencyAnalysis struct {
	CollectionID    string                     `json:"collection_id"`
	TimeRange       TimeRange                  `json:"time_range"`
	FrequentPaths   []PathFrequency            `json:"frequent_paths"`
	VolatileEndpoints []EndpointVolatility     `json:"volatile_endpoints"`
}

type PathFrequency struct {
	Path        string  `json:"path"`
	HumanPath   string  `json:"human_path"`
	Count       int     `json:"count"`
	Percentage  float64 `json:"percentage"`
	LastChanged time.Time `json:"last_changed"`
}

type EndpointVolatility struct {
	EndpointName string    `json:"endpoint_name"`
	ChangeCount  int       `json:"change_count"`
	LastChanged  time.Time `json:"last_changed"`
	ChangeTypes  map[string]int `json:"change_types"`
}

// AnalyzeChangeImpact performs impact analysis on changes
func AnalyzeChangeImpact(collectionID string, snapshotID int64) (*ChangeImpactAnalysis, error) {
	// Get all changes for the snapshot
	filter := ChangeFilter{
		CollectionID: collectionID,
		SnapshotID:   &snapshotID,
		Limit:        10000,
	}
	
	changes, _, err := GetChanges(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get changes: %w", err)
	}
	
	analysis := &ChangeImpactAnalysis{
		CollectionID:    collectionID,
		SnapshotID:      snapshotID,
		BreakingChanges: make([]*ImpactDetail, 0),
		SecurityChanges: make([]*ImpactDetail, 0),
		DataChanges:     make([]*ImpactDetail, 0),
		CosmeticChanges: make([]*ImpactDetail, 0),
	}
	
	for _, change := range changes {
		impactDetail := analyzeIndividualChange(change)
		
		switch impactDetail.Severity {
		case "breaking":
			analysis.BreakingChanges = append(analysis.BreakingChanges, impactDetail)
		case "security":
			analysis.SecurityChanges = append(analysis.SecurityChanges, impactDetail)
		case "data":
			analysis.DataChanges = append(analysis.DataChanges, impactDetail)
		default:
			analysis.CosmeticChanges = append(analysis.CosmeticChanges, impactDetail)
		}
	}
	
	// Calculate summary
	analysis.Summary = calculateImpactSummary(analysis)
	
	return analysis, nil
}

func analyzeIndividualChange(change *ChangeDetail) *ImpactDetail {
	impact := &ImpactDetail{
		Change:      change,
		Severity:    "low",
		Suggestions: make([]string, 0),
	}
	
	// Analyze based on path and change type
	switch {
	// Breaking changes
	case change.ChangeType == "deleted" && strings.Contains(change.Path, ".item["):
		impact.Severity = "breaking"
		impact.Impact = "Endpoint removed - clients using this endpoint will fail"
		impact.Suggestions = append(impact.Suggestions, "Notify all API consumers about endpoint removal")
		impact.Suggestions = append(impact.Suggestions, "Consider deprecation period before removal")
		
	case change.ChangeType == "deleted" && strings.Contains(change.Path, ".response"):
		impact.Severity = "breaking"
		impact.Impact = "Response structure changed - may break client parsing"
		impact.Suggestions = append(impact.Suggestions, "Version the API to maintain backward compatibility")
		
	case change.ChangeType == "modified" && strings.Contains(change.Path, ".url"):
		impact.Severity = "breaking"
		impact.Impact = "URL changed - existing integrations will fail"
		impact.Suggestions = append(impact.Suggestions, "Implement URL redirects if possible")
		impact.Suggestions = append(impact.Suggestions, "Update all documentation and client code")
		
	// Security changes
	case strings.Contains(change.Path, "auth") || strings.Contains(change.Path, "authorization"):
		impact.Severity = "security"
		impact.Impact = "Authentication/Authorization change detected"
		impact.Suggestions = append(impact.Suggestions, "Review security implications")
		impact.Suggestions = append(impact.Suggestions, "Test all authentication flows")
		
	case strings.Contains(change.Path, "header") && change.ChangeType == "deleted":
		impact.Severity = "security"
		impact.Impact = "Header removed - may affect security headers"
		impact.Suggestions = append(impact.Suggestions, "Verify no security headers were removed")
		
	// Data changes
	case strings.Contains(change.Path, ".body") || strings.Contains(change.Path, ".raw"):
		impact.Severity = "data"
		impact.Impact = "Request/Response body structure modified"
		impact.Suggestions = append(impact.Suggestions, "Update API documentation")
		impact.Suggestions = append(impact.Suggestions, "Test data validation")
		
	case change.ChangeType == "added" && strings.Contains(change.Path, ".response"):
		impact.Severity = "data"
		impact.Impact = "New response added - additional test coverage needed"
		impact.Suggestions = append(impact.Suggestions, "Add tests for new response scenario")
		
	// Cosmetic changes
	case strings.Contains(change.Path, ".name") || strings.Contains(change.Path, ".description"):
		impact.Severity = "low"
		impact.Impact = "Documentation/naming change only"
		
	default:
		impact.Severity = "low"
		impact.Impact = "Minor change with minimal impact"
	}
	
	return impact
}

func calculateImpactSummary(analysis *ChangeImpactAnalysis) ImpactSummary {
	summary := ImpactSummary{
		TotalBreaking: len(analysis.BreakingChanges),
		TotalSecurity: len(analysis.SecurityChanges),
		TotalData:     len(analysis.DataChanges),
		TotalCosmetic: len(analysis.CosmeticChanges),
	}
	
	// Calculate risk score (0-100)
	summary.RiskScore = float64(summary.TotalBreaking*40 + summary.TotalSecurity*30 + 
		summary.TotalData*20 + summary.TotalCosmetic*1)
	
	// Normalize to 0-100
	totalChanges := summary.TotalBreaking + summary.TotalSecurity + summary.TotalData + summary.TotalCosmetic
	if totalChanges > 0 {
		summary.RiskScore = (summary.RiskScore / float64(totalChanges)) * 2
		if summary.RiskScore > 100 {
			summary.RiskScore = 100
		}
	}
	
	// Generate recommendation
	switch {
	case summary.RiskScore >= 80:
		summary.Recommendation = "CRITICAL: Major breaking changes detected. Extensive testing and communication required."
	case summary.RiskScore >= 60:
		summary.Recommendation = "HIGH RISK: Significant changes detected. Thorough testing recommended before deployment."
	case summary.RiskScore >= 40:
		summary.Recommendation = "MODERATE: Notable changes present. Standard testing procedures should suffice."
	case summary.RiskScore >= 20:
		summary.Recommendation = "LOW RISK: Minor changes detected. Basic smoke testing recommended."
	default:
		summary.Recommendation = "MINIMAL: Mostly cosmetic changes. Safe to deploy after basic verification."
	}
	
	return summary
}

// GetChangeFrequencyAnalysis analyzes which paths change most frequently
func GetChangeFrequencyAnalysis(collectionID string, days int) (*ChangeFrequencyAnalysis, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)
	
	// Query for path frequencies
	query := `
		SELECT 
			path,
			COUNT(*) as count,
			MAX(created_at) as last_changed
		FROM changes
		WHERE collection_id = $1
			AND created_at >= $2
			AND created_at <= $3
		GROUP BY path
		ORDER BY count DESC
		LIMIT 20
	`
	
	rows, err := DB.Query(query, collectionID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query frequency: %w", err)
	}
	defer rows.Close()
	
	analysis := &ChangeFrequencyAnalysis{
		CollectionID: collectionID,
		TimeRange: TimeRange{
			Earliest: startTime,
			Latest:   endTime,
		},
		FrequentPaths: make([]PathFrequency, 0),
	}
	
	totalChanges := 0
	paths := make([]PathFrequency, 0)
	
	for rows.Next() {
		var pf PathFrequency
		err := rows.Scan(&pf.Path, &pf.Count, &pf.LastChanged)
		if err != nil {
			continue
		}
		
		pf.HumanPath = generateHumanPath(parsePathSegments(pf.Path))
		totalChanges += pf.Count
		paths = append(paths, pf)
	}
	
	// Calculate percentages
	for i := range paths {
		paths[i].Percentage = (float64(paths[i].Count) / float64(totalChanges)) * 100
	}
	
	analysis.FrequentPaths = paths
	
	// Get volatile endpoints
	analysis.VolatileEndpoints, err = getVolatileEndpoints(collectionID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	
	return analysis, nil
}

func getVolatileEndpoints(collectionID string, startTime, endTime time.Time) ([]EndpointVolatility, error) {
	// Query for endpoint volatility
	query := `
		SELECT 
			COALESCE(
				SUBSTRING(path FROM 'collection\.item\[(\d+)\]'),
				'Collection Settings'
			) as endpoint_key,
			COUNT(*) as change_count,
			MAX(created_at) as last_changed,
			change_type,
			COUNT(*) as type_count
		FROM changes
		WHERE collection_id = $1
			AND created_at >= $2
			AND created_at <= $3
			AND path LIKE 'collection.item[%]%'
		GROUP BY endpoint_key, change_type
	`
	
	rows, err := DB.Query(query, collectionID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query volatility: %w", err)
	}
	defer rows.Close()
	
	// Group by endpoint
	endpointMap := make(map[string]*EndpointVolatility)
	
	for rows.Next() {
		var endpointKey string
		var changeType string
		var typeCount int
		var lastChanged time.Time
		var totalCount int
		
		err := rows.Scan(&endpointKey, &totalCount, &lastChanged, &changeType, &typeCount)
		if err != nil {
			continue
		}
		
		if _, exists := endpointMap[endpointKey]; !exists {
			endpointMap[endpointKey] = &EndpointVolatility{
				EndpointName: fmt.Sprintf("Endpoint %s", endpointKey),
				ChangeTypes:  make(map[string]int),
				LastChanged:  lastChanged,
			}
		}
		
		ev := endpointMap[endpointKey]
		ev.ChangeCount += typeCount
		ev.ChangeTypes[changeType] = typeCount
		if lastChanged.After(ev.LastChanged) {
			ev.LastChanged = lastChanged
		}
	}
	
	// Convert to slice
	volatility := make([]EndpointVolatility, 0, len(endpointMap))
	for _, ev := range endpointMap {
		volatility = append(volatility, *ev)
	}
	
	// Sort by change count
	sort.Slice(volatility, func(i, j int) bool {
		return volatility[i].ChangeCount > volatility[j].ChangeCount
	})
	
	return volatility, nil
}

// CompareSnapshots generates a detailed comparison between two snapshots
func CompareSnapshots(collectionID string, oldSnapshotID, newSnapshotID int64) (map[string]interface{}, error) {
	// Get changes between snapshots
	query := `
		SELECT 
			change_type,
			path,
			modification,
			created_at
		FROM changes
		WHERE collection_id = $1
			AND old_snapshot_id = $2
			AND new_snapshot_id = $3
		ORDER BY path
	`
	
	rows, err := DB.Query(query, collectionID, oldSnapshotID, newSnapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to query comparison: %w", err)
	}
	defer rows.Close()
	
	comparison := map[string]interface{}{
		"collection_id":    collectionID,
		"old_snapshot_id":  oldSnapshotID,
		"new_snapshot_id":  newSnapshotID,
		"changes":          make([]map[string]interface{}, 0),
		"statistics":       make(map[string]int),
		"affected_paths":   make(map[string][]string),
	}
	
	stats := comparison["statistics"].(map[string]int)
	affectedPaths := comparison["affected_paths"].(map[string][]string)
	changes := make([]map[string]interface{}, 0)
	
	for rows.Next() {
		var changeType, path string
		var modification *string
		var createdAt time.Time
		
		err := rows.Scan(&changeType, &path, &modification, &createdAt)
		if err != nil {
			continue
		}
		
		// Update statistics
		stats[changeType]++
		stats["total"]++
		
		// Track affected paths by type
		resourceType := extractResourceType(path)
		affectedPaths[resourceType] = append(affectedPaths[resourceType], path)
		
		// Create change entry
		change := map[string]interface{}{
			"type":       changeType,
			"path":       path,
			"human_path": generateHumanPath(parsePathSegments(path)),
			"created_at": createdAt,
		}
		
		if modification != nil {
			// Try to parse modification
			var parsed interface{}
			if err := json.Unmarshal([]byte(*modification), &parsed); err == nil {
				change["modification"] = parsed
			} else {
				change["modification"] = *modification
			}
		}
		
		changes = append(changes, change)
	}
	
	comparison["changes"] = changes
	
	// Add metadata about both snapshots
	snapshotQuery := `
		SELECT id, content_hash, created_at
		FROM snapshots
		WHERE id IN ($1, $2)
	`
	
	snapshotRows, err := DB.Query(snapshotQuery, oldSnapshotID, newSnapshotID)
	if err == nil {
		defer snapshotRows.Close()
		
		snapshots := make(map[int64]map[string]interface{})
		for snapshotRows.Next() {
			var id int64
			var hash string
			var createdAt time.Time
			
			if err := snapshotRows.Scan(&id, &hash, &createdAt); err == nil {
				snapshots[id] = map[string]interface{}{
					"id":         id,
					"hash":       hash,
					"created_at": createdAt,
				}
			}
		}
		
		comparison["old_snapshot"] = snapshots[oldSnapshotID]
		comparison["new_snapshot"] = snapshots[newSnapshotID]
	}
	
	return comparison, nil
}

func EnhanceChangeDetail(change *ChangeDetail) {
	// Parse path into segments
	change.PathSegments = parsePathSegments(change.Path)

	// Generate human-readable path
	change.HumanPath = generateHumanPath(change.PathSegments)

	// Extract endpoint name and resource type
	change.EndpointName = extractEndpointName(change.Path, change.Modification)
	change.ResourceType = extractResourceType(change.Path)
}