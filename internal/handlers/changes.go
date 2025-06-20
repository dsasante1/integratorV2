package handlers

import (
	"integratorV2/internal/db"
	"net/http"
	"log/slog"
	"strconv"
	"time"
	"fmt"
	"encoding/json"
	"github.com/labstack/echo/v4"
)


type DiffRequest struct {
	CollectionID string `param:"collectionId"`
	SnapshotID   string `param:"snapshotId"`
	// Query parameters
	Search     string `query:"search"`
	FilterType string `query:"filter_type"` // all, added, deleted, modified
	GroupBy    string `query:"group_by"`    // none, endpoint, type
	Page       int    `query:"page"`
	PageSize   int    `query:"page_size"`
	SortBy     string `query:"sort_by"`     // endpoint, type, path
	SortOrder  string `query:"sort_order"`  // asc, desc
}

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

type DiffResponse struct {
	OldSnapshotID int64        `json:"old_snapshot_id"`
	NewSnapshotID int64        `json:"new_snapshot_id"`
	CollectionID  string       `json:"collection_id"`
	Changes       []DiffDetail `json:"changes"`
	Summary       DiffSummary  `json:"summary"`
}

func GetCollectionChangeSummary(c echo.Context) error {
		collectionID := c.Param("collectionId")

		summary, err := db.GetCollectionChangeSummary(collectionID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	
	return c.JSON(http.StatusOK, summary)
}

 
func GetChangeSummary(c echo.Context) error {
	collectionID := c.Param("collectionId")
	
	 
	oldSnapshotStr := c.QueryParam("oldSnapshot")
	newSnapshotStr := c.QueryParam("newSnapshot")
	
	var oldSnapshotID *int64
	if oldSnapshotStr != "" {
		id, err := strconv.ParseInt(oldSnapshotStr, 10, 64)
		if err != nil {
		slog.Warn("invalid snapshot id", "error", oldSnapshotID)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid old snapshot ID")

		}
		oldSnapshotID = &id
	}

	var newSnapshotID *int64
	if newSnapshotStr != "" {
		id, err := strconv.ParseInt(newSnapshotStr, 10, 64)
		if err != nil {
		slog.Warn("invalid snapshot id", "error", newSnapshotID)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid new snapshot ID")
		}
	

	newSnapshotID = &id
	}
	summary, err := db.GetChangeSummary(collectionID, oldSnapshotID, newSnapshotID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	
	return c.JSON(http.StatusOK, summary)
}

 
func GetChanges(c echo.Context) error {
	collectionID := c.Param("collectionId")
	
	 
	filter := db.ChangeFilter{
		CollectionID: collectionID,
		Limit:        50,  // Default limit
		Offset:       0,
	}
	
	 
	if limit := c.QueryParam("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 && l <= 100 {
			filter.Limit = l
		}
	}
	
	if offset := c.QueryParam("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filter.Offset = o
		}
	}
	
	 
	if changeTypes := c.QueryParams()["type"]; len(changeTypes) > 0 {
		filter.ChangeTypes = changeTypes
	}
	
	if pattern := c.QueryParam("path"); pattern != "" {
		filter.PathPattern = pattern
	}
	
	if snapshot := c.QueryParam("snapshot"); snapshot != "" {
		if id, err := strconv.ParseInt(snapshot, 10, 64); err == nil {
			filter.SnapshotID = &id
		}
	}
	
	 
	if start := c.QueryParam("startTime"); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			filter.StartTime = &t
		}
	}
	
	if end := c.QueryParam("endTime"); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			filter.EndTime = &t
		}
	}
	
	changes, total, err := db.GetChanges(filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	
	response := map[string]interface{}{
		"changes": changes,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	}
	
	return c.JSON(http.StatusOK, response)
}

 
func GetChangeHierarchy(c echo.Context) error {
	collectionID := c.Param("collectionId")
	snapshotID := c.Param("snapshotId")
	
	snapshot, err := strconv.ParseInt(snapshotID, 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid snapshot ID")
	}
	
	hierarchy, err := db.GetChangeHierarchy(collectionID, snapshot)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	
	return c.JSON(http.StatusOK, hierarchy)
}

 
func GetChangesByEndpoint(c echo.Context) error {
	collectionID := c.Param("collectionId")
	snapshotID := c.Param("snapshotId")
	
	snapshot, err := strconv.ParseInt(snapshotID, 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid snapshot ID")
	}
	
	endpointChanges, err := db.GetChangesByEndpoint(collectionID, snapshot)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	
	return c.JSON(http.StatusOK, endpointChanges)
}

 
func GetChangeDetails(c echo.Context) error {
	changeID := c.Param("changeId")
	
	id, err := strconv.ParseInt(changeID, 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid change ID")
	}

	change, err := db.GetChangeDetails(id)
	
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "get change details failed")
	}


	if change.Modification != nil {
		var parsed interface{}
		if err := json.Unmarshal([]byte(*change.Modification), &parsed); err == nil {
			response := map[string]interface{}{
				"change": change,
				"parsedModification": parsed,
			}
			return c.JSON(http.StatusOK, response)
		}
	}
	
	return c.JSON(http.StatusOK, change)
}

 
func ExportChanges(c echo.Context) error {
	collectionID := c.Param("collectionId")
	format := c.QueryParam("format")
	
	if format == "" {
		format = "json"
	}
	
	 
	filter := db.ChangeFilter{
		CollectionID: collectionID,
		Limit:        10000, // Get all for export
	}
	
	if snapshot := c.QueryParam("snapshot"); snapshot != "" {
		if id, err := strconv.ParseInt(snapshot, 10, 64); err == nil {
			filter.SnapshotID = &id
		}
	}
	
	changes, _, err := db.GetChanges(filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	
	switch format {
	case "json":
		c.Response().Header().Set("Content-Type", "application/json")
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"changes_%s.json\"", collectionID))
		return c.JSON(http.StatusOK, changes)
		
	case "csv":
		c.Response().Header().Set("Content-Type", "text/csv")
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"changes_%s.csv\"", collectionID))
		
		// Write CSV header
		fmt.Fprintln(c.Response().Writer, "ID,Type,Path,Human Path,Endpoint,Resource Type,Created At,Modification")
		
		// Write data rows
		for _, change := range changes {
			mod := ""
			if change.Modification != nil {
				mod = *change.Modification
			}
			fmt.Fprintf(c.Response().Writer, "%d,%s,%s,%s,%s,%s,%s,%q\n",
				change.ID,
				change.ChangeType,
				change.Path,
				change.HumanPath,
				change.EndpointName,
				change.ResourceType,
				change.CreatedAt.Format(time.RFC3339),
				mod,
			)
		}
		return nil
		
	case "markdown":
		c.Response().Header().Set("Content-Type", "text/markdown")
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"changes_%s.md\"", collectionID))
		
		// Group by endpoint for markdown
		endpointChanges, _ := db.GetChangesByEndpoint(collectionID, *filter.SnapshotID)
		
		fmt.Fprintf(c.Response().Writer, "# Change Report for Collection %s\n\n", collectionID)
		fmt.Fprintf(c.Response().Writer, "Generated: %s\n\n", time.Now().Format(time.RFC3339))
		
		for endpoint, changes := range endpointChanges {
			fmt.Fprintf(c.Response().Writer, "## %s\n\n", endpoint)
			for _, change := range changes {
				fmt.Fprintf(c.Response().Writer, "- **%s**: %s\n", change.ChangeType, change.HumanPath)
				if change.Modification != nil && len(*change.Modification) < 100 {
					fmt.Fprintf(c.Response().Writer, "  - Value: `%s`\n", *change.Modification)
				}
			}
			fmt.Fprintln(c.Response().Writer)
		}
		return nil
		
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "Unsupported format")
	}
}

 
func GetChangeImpactAnalysis(c echo.Context) error {
	collectionID := c.Param("collectionId")
	snapshotID := c.Param("snapshotId")
	
	snapshot, err := strconv.ParseInt(snapshotID, 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid snapshot ID")
	}
	
	analysis, err := db.AnalyzeChangeImpact(collectionID, snapshot)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	
	return c.JSON(http.StatusOK, analysis)
}

 
func GetChangeFrequencyAnalysis(c echo.Context) error {
	collectionID := c.Param("collectionId")
	daysStr := c.QueryParam("days")
	
	days := 30 // Default to 30 days
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}
	
	analysis, err := db.GetChangeFrequencyAnalysis(collectionID, days)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	
	return c.JSON(http.StatusOK, analysis)
}

 
func CompareSnapshots(c echo.Context) error {
	collectionID := c.Param("collectionId")
	oldSnapshotStr := c.QueryParam("old")
	newSnapshotStr := c.QueryParam("new")
	
	if oldSnapshotStr == "" || newSnapshotStr == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Both 'old' and 'new' snapshot IDs are required")
	}
	
	oldSnapshotID, err := strconv.ParseInt(oldSnapshotStr, 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid old snapshot ID")
	}
	
	newSnapshotID, err := strconv.ParseInt(newSnapshotStr, 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid new snapshot ID")
	}
	
	comparison, err := db.CompareSnapshots(collectionID, oldSnapshotID, newSnapshotID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	
	return c.JSON(http.StatusOK, comparison)
}


func GetSnapshotDiff(c echo.Context) error {
	var req db.DiffRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request parameters"})
	}

	if req.CollectionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID is required"})
	}

	if req.SnapshotID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Snapshot ID is required"})
	}

	validateSnapshotID, err := strconv.ParseInt(req.SnapshotID, 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid snapshot ID")
	}

	// Set defaults
	if req.PageSize == 0 {
		req.PageSize = 50 // Default page size
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.FilterType == "" {
		req.FilterType = "all"
	}
	if req.SortOrder == "" {
		req.SortOrder = "asc"
	}

	result, err := db.GetFilteredSnapshotDiff(req.CollectionID, validateSnapshotID, req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "fetch snapshot diff failed: "+err.Error())
	}

	return c.JSON(http.StatusOK, result)
}

func GetSnapshotDiffID(c echo.Context) error {
	collectionID := c.Param("collectionId")

	if collectionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID is required"})
	}

	snapshotID, err := db.GetSnapshotDiffID(collectionID)

	if err != nil {
		slog.Error("failed to fetch snapshot IDs", "error", err)
		return c.JSON(http.StatusNotFound,
		map[string]string{"error": "Snapshot IDs not found"})
	}

	return c.JSON(http.StatusOK, snapshotID)
}


func SearchEndpoints(c echo.Context) error {
	collectionID := c.Param("collectionId")
	search := c.QueryParam("q")
	
	if collectionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID is required"})
	}
	
	endpoints, err := db.SearchEndpoints(collectionID, search)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to search endpoints")
	}
	
	return c.JSON(http.StatusOK, endpoints)
}


func GetChangeDetail(c echo.Context) error {
	collectionID := c.Param("collectionId")
	changeID := c.Param("changeId")
	
	if collectionID == "" || changeID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID and Change ID are required"})
	}
	
	changeIDInt, err := strconv.ParseInt(changeID, 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid change ID")
	}
	
	change, err := db.GetChangeDetail(collectionID, changeIDInt)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Change not found")
	}
	
	return c.JSON(http.StatusOK, change)
}
