package autotasktest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// parseEntityAndID extracts entity name and optional ID from path.
// Paths: /v1.0/{entity}/{id} or /v1.0/{parent}/{parentID}/{child}
func parseEntityAndID(path string) (entityName string, id int64, isChild bool) {
	// Strip leading "/v1.0/"
	trimmed := strings.TrimPrefix(path, "/v1.0/")
	parts := strings.Split(trimmed, "/")
	switch len(parts) {
	case 1:
		// /v1.0/{entity}
		entityName = parts[0]
	case 2: //nolint:mnd // path segment count
		// /v1.0/{entity}/{id}
		entityName = parts[0]
		id, _ = strconv.ParseInt(parts[1], 10, 64)
	case 3: //nolint:mnd // path segment count
		// /v1.0/{parent}/{parentID}/{child}
		entityName = parts[2]
		isChild = true
	default:
		entityName = parts[0]
	}
	return
}

func (ts *TestServer) getStore(entityName string) (*entityStore, bool) {
	store, ok := ts.entities[entityName]
	return store, ok
}

func (ts *TestServer) handleGet(w http.ResponseWriter, r *http.Request) {
	entityName, id, isChild := parseEntityAndID(r.URL.Path)
	store, ok := ts.getStore(entityName)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, []string{fmt.Sprintf("Entity %q not found", entityName)})
		return
	}

	if isChild || id == 0 {
		// Child entity listing or bare entity GET — return all items.
		items := store.all()
		pageSize := ts.opts.pageSize
		if pageSize > len(items) {
			pageSize = len(items)
		}
		page := items[:pageSize]
		var nextPageURL string
		if len(items) > pageSize {
			nextPageURL = fmt.Sprintf("%s%s?page=2", ts.URL, r.URL.Path)
		}
		writeJSON(w, map[string]any{
			"items": page,
			"pageDetails": map[string]any{
				"count":        len(page),
				"requestCount": ts.opts.pageSize,
				"prevPageUrl":  nil,
				"nextPageUrl":  nextPageURL,
			},
		})
		return
	}

	item, found := store.getByID(id)
	if !found {
		writeErrorResponse(w, http.StatusNotFound, []string{fmt.Sprintf("%s with ID %d not found", entityName, id)})
		return
	}
	writeJSON(w, map[string]any{"item": item})
}

func (ts *TestServer) handleCreate(w http.ResponseWriter, r *http.Request) {
	entityName, _, isChild := parseEntityAndID(r.URL.Path)
	store, ok := ts.getStore(entityName)
	if !ok {
		// Auto-create store for unknown entities.
		store = newEntityStore(entityName)
		ts.entities[entityName] = store
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, []string{"Failed to read request body"})
		return
	}

	// Validate required fields.
	if len(store.requiredFields) > 0 {
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, []string{"Invalid JSON body"})
			return
		}
		for _, field := range store.requiredFields {
			if _, exists := m[field]; !exists {
				writeErrorResponse(w, http.StatusBadRequest, []string{fmt.Sprintf("Required field %q is missing", field)})
				return
			}
		}
	}

	// Assign ID and store.
	newID := store.allocateID()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, []string{"Invalid JSON body"})
		return
	}
	m["id"] = newID
	data, err := json.Marshal(m)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, []string{"Failed to marshal entity"})
		return
	}
	store.add(data)

	_ = isChild // child creates work the same way
	writeJSON(w, map[string]any{"itemId": newID})
}

func (ts *TestServer) handleUpdate(w http.ResponseWriter, r *http.Request) {
	entityName, _, _ := parseEntityAndID(r.URL.Path)
	store, ok := ts.getStore(entityName)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, []string{fmt.Sprintf("Entity %q not found", entityName)})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, []string{"Failed to read request body"})
		return
	}

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, []string{"Invalid JSON body"})
		return
	}

	id := extractID(m)
	if id == 0 {
		writeErrorResponse(w, http.StatusBadRequest, []string{"'id' field is required for update"})
		return
	}

	if _, found := store.getByID(id); !found {
		writeErrorResponse(w, http.StatusNotFound, []string{fmt.Sprintf("%s with ID %d not found", entityName, id)})
		return
	}

	// Return {"itemId": N} matching real Autotask PATCH behavior.
	writeJSON(w, map[string]any{"itemId": id})
}

func (ts *TestServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	entityName, id, _ := parseEntityAndID(r.URL.Path)
	store, ok := ts.getStore(entityName)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, []string{fmt.Sprintf("Entity %q not found", entityName)})
		return
	}

	if !store.canDelete {
		writeErrorResponse(w, http.StatusBadRequest, []string{fmt.Sprintf("Entity %q does not support deletion", entityName)})
		return
	}

	if !store.deleteByID(id) {
		writeErrorResponse(w, http.StatusNotFound, []string{fmt.Sprintf("%s with ID %d not found", entityName, id)})
		return
	}

	writeJSON(w, map[string]any{"itemId": id})
}

// Metadata handlers.

func (ts *TestServer) handleEntityInfo(w http.ResponseWriter, r *http.Request) {
	// Extract entity name from path like /v1.0/Companies/entityInformation
	trimmed := strings.TrimPrefix(r.URL.Path, "/v1.0/")
	parts := strings.SplitN(trimmed, "/", 2) //nolint:mnd // split into at most 2 parts
	entityName := parts[0]

	md, ok := ts.opts.metadata[entityName]
	if !ok || md.info == nil {
		writeJSON(w, EntityInfoResponse{Name: entityName, CanCreate: true, CanUpdate: true, CanQuery: true})
		return
	}
	writeJSON(w, md.info)
}

func (ts *TestServer) handleEntityFields(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/v1.0/")
	parts := strings.SplitN(trimmed, "/", 2) //nolint:mnd // split into at most 2 parts
	entityName := parts[0]

	md, ok := ts.opts.metadata[entityName]
	if !ok || md.fields == nil {
		writeJSON(w, map[string]any{"fields": []any{}})
		return
	}
	writeJSON(w, map[string]any{"fields": md.fields})
}

func (ts *TestServer) handleEntityUDFs(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/v1.0/")
	parts := strings.SplitN(trimmed, "/", 2) //nolint:mnd // split into at most 2 parts
	entityName := parts[0]

	md, ok := ts.opts.metadata[entityName]
	if !ok || md.udfs == nil {
		writeJSON(w, map[string]any{"fields": []any{}})
		return
	}
	writeJSON(w, map[string]any{"fields": md.udfs})
}

//nolint:unparam // r is required by http.Handler signature
func (ts *TestServer) handleZoneInfo(w http.ResponseWriter, r *http.Request) {
	zone := ts.opts.zoneInfo
	if zone == nil {
		writeJSON(w, map[string]any{
			"zoneName": "Test Zone",
			"url":      ts.URL,
			"webUrl":   ts.URL,
			"ci":       1,
		})
		return
	}
	writeJSON(w, map[string]any{
		"zoneName": zone.name,
		"url":      zone.url,
		"webUrl":   zone.url,
		"ci":       1,
	})
}

//nolint:unparam // r is required by http.Handler signature
func (ts *TestServer) handleVersionInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"versions": []string{"1.0"}})
}

// Threshold request count constants for test responses.
const (
	thresholdExternalCurrentCount = 100
	thresholdExternalLimit        = 10000
	thresholdInternalCurrentCount = 50
	thresholdInternalLimit        = 10000
)

//nolint:unparam // r is required by http.Handler signature
func (ts *TestServer) handleThresholdInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"currentTimeframeExternalCrossDomainRequestCount": thresholdExternalCurrentCount,
		"externalCrossDomainRequestThreshold":             thresholdExternalLimit,
		"currentTimeframeInternalCrossDomainRequestCount": thresholdInternalCurrentCount,
		"internalCrossDomainRequestThreshold":             thresholdInternalLimit,
	})
}
