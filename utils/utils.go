package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"log/slog"
	"encoding/json"
	"strings"
	"integratorV2/internal/db"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GenerateRandomAlphaNumeric(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("length must be greater than 0")
	}

	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := 0; i < length; i++ {
		randomIndex, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			slog.Error("failed to generate random number", "error", err)
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		result[i] = charset[randomIndex.Int64()]
	}

	return string(result), nil
}



type Change struct {
	ID             int      `json:"id"`
	CollectionID   string   `json:"collection_id"`
	OldSnapshotID  int      `json:"old_snapshot_id"`
	NewSnapshotID  int      `json:"new_snapshot_id"`
	ChangeType     string   `json:"change_type"`
	Path           string   `json:"path"`
	Modification   string   `json:"modification"`
	CreatedAt      string   `json:"created_at"`
	HumanPath      string   `json:"human_path"`
	PathSegments   []string `json:"path_segments"`
	EndpointName   string   `json:"endpoint_name"`
	ResourceType   string   `json:"resource_type"`
	OldValue       *string  `json:"old_value"` // Using pointer to detect null
	NewValue       *string  `json:"new_value"` // Using pointer to detect null
}

// Summary represents the summary object
type Summary struct {
	TotalChanges     int                `json:"total_changes"`
	ChangesByType    map[string]int     `json:"changes_by_type"`
	AffectedEndpoints []string          `json:"affected_endpoints"`
}


type Pagination struct {
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalItems int  `json:"total_items"`
	TotalPages int  `json:"total_pages"`
	HasMore    bool `json:"has_more"`
}


type ChangeLog struct {
	OldSnapshotID int        `json:"old_snapshot_id"`
	NewSnapshotID int        `json:"new_snapshot_id"`
	CollectionID  string     `json:"collection_id"`
	Changes       []Change   `json:"changes"`
	Summary       Summary    `json:"summary"`
	Pagination    Pagination `json:"pagination"`
}

// FilterChangesWithNullValues filters out change objects where both old_value and new_value are null
func FilterChangesWithNullValues(jsonData []byte) ([]byte, error) {
	var changeLog ChangeLog
	
	
	err := json.Unmarshal(jsonData, &changeLog)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %v", err)
	}
	
	
	var filteredChanges []Change
	affectedEndpoints := make(map[string]bool)
	
	for _, change := range changeLog.Changes {
		
		if change.OldValue != nil || change.NewValue != nil {
			filteredChanges = append(filteredChanges, change)
			affectedEndpoints[change.EndpointName] = true
		}
	}
	
	
	changeLog.Changes = filteredChanges
	
	
	changeLog.Summary.TotalChanges = len(filteredChanges)
	changeLog.Summary.ChangesByType = make(map[string]int)
	
	
	for _, change := range filteredChanges {
		changeLog.Summary.ChangesByType[change.ChangeType]++
	}
	
	
	var endpointNames []string
	for endpoint := range affectedEndpoints {
		endpointNames = append(endpointNames, endpoint)
	}
	changeLog.Summary.AffectedEndpoints = endpointNames
	
	
	changeLog.Pagination.TotalItems = len(filteredChanges)
	if changeLog.Pagination.PageSize > 0 {
		changeLog.Pagination.TotalPages = (len(filteredChanges) + changeLog.Pagination.PageSize - 1) / changeLog.Pagination.PageSize
	}
	changeLog.Pagination.HasMore = changeLog.Pagination.Page < changeLog.Pagination.TotalPages
	
	
	filteredJSON, err := json.MarshalIndent(changeLog, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("error marshaling filtered JSON: %v", err)
	}
	
	return filteredJSON, nil
}
	


// CleanJSONContent removes backslashes and replaces \n with actual newlines
func CleanJSONContent(content string) string {
	
	cleaned := strings.ReplaceAll(content, "\\\"", "\"")
	
	
	cleaned = strings.ReplaceAll(cleaned, "\\n", " ")
	
	
	cleaned = strings.ReplaceAll(cleaned, "\\", "")
	
	return cleaned
}


func CleanSpecificFields(content string) string {
	
	
	
	lines := strings.Split(content, "\n")
	var result []string
	
	for _, line := range lines {
		if strings.Contains(line, "\"modification\":") {
			
			parts := strings.SplitN(line, "\"modification\":", 2)
			if len(parts) == 2 {
				prefix := parts[0] + "\"modification\":"
				value := strings.TrimSpace(parts[1])
				
				
				hasComma := strings.HasSuffix(value, ",")
				if hasComma {
					value = strings.TrimSuffix(value, ",")
				}
				
				
				cleanedValue := CleanJSONContent(value)
				
				
				if hasComma {
					line = prefix + " " + cleanedValue + ","
				} else {
					line = prefix + " " + cleanedValue
				}
			}
		}
		result = append(result, line)
	}
	
	return strings.Join(result, "\n")
}


func HandleDiffResponse(result db.PaginatedDiffResponse) (string, error) {
    jsonData, err := json.Marshal(result)
    if err != nil {
        return "", err
    }
    
    filteredData, err := FilterChangesWithNullValues(jsonData)
    if err != nil {
        return "", err
    }
    
    cleanedData := CleanSpecificFields(string(filteredData))
    return cleanedData, nil
}