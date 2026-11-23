package autotasktest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// requestPath is a parsed request path.
type requestPath struct {
	entity   string // entity name, or the child segment for a child path
	id       int64  // item id, or 0 when the path names a collection
	child    bool   // the path is under a parent
	parent   string // parent entity name, for a child path
	parentID int64  // parent id, for a child path
}

// parsePath splits a request path. Paths:
//
//	/v1.0/{entity}
//	/v1.0/{entity}/{id}
//	/v1.0/{parent}/{parentID}/{child}
//	/v1.0/{parent}/{parentID}/{child}/{id}
//
// Any longer path is read as /v1.0/{entity}.
func parsePath(path string) requestPath {
	parts := strings.Split(strings.TrimPrefix(path, "/v1.0/"), "/")
	var p requestPath
	switch len(parts) {
	case 1:
		p.entity = parts[0]
	case 2: //nolint:mnd // path segment count
		p.entity = parts[0]
		p.id, _ = strconv.ParseInt(parts[1], 10, 64)
	case 3, 4: //nolint:mnd // path segment count
		p.child = true
		p.parent = parts[0]
		p.parentID, _ = strconv.ParseInt(parts[1], 10, 64)
		p.entity = parts[2]
		if len(parts) == 4 { //nolint:mnd // path segment count
			p.id, _ = strconv.ParseInt(parts[3], 10, 64)
		}
	default:
		p.entity = parts[0]
	}
	return p
}

// singularEntityName returns the singular form of an Autotask entity name, with
// the same rules as the singular function of cmd/autotask-gen: two irregular
// plurals, a trailing "ies" becomes "y", else a trailing "s" is dropped. The
// mock relies on the naming convention that a child collection
// /{Parent}/{id}/{Segment} holds the entities named singular(Parent)+Segment,
// as in Tickets/Attachments and TicketAttachments. The convention is read off
// the entity types in this module, it is not an Autotask guarantee.
func singularEntityName(s string) string {
	switch s {
	case "Statuses":
		return "Status"
	case "Addresses":
		return "Address"
	}
	if strings.HasSuffix(s, "ies") {
		return s[:len(s)-3] + "y"
	}
	return strings.TrimSuffix(s, "s")
}

// resolveChildStore finds the store behind a child URL
// /{parent}/{id}/{segment}. With derived = singular(parent)+segment, in order:
//
//  1. the route registered by WithChildEntity;
//  2. the store named derived, when it exists;
//  3. derived itself when it is the entity name of a type in package entities
//     (knownEntityNames): a create makes it, a read reports false when it is
//     missing, and the store named segment is never used;
//  4. the store named segment, when it exists;
//  5. on a create, the store named segment when that is the entity name of a
//     type in package entities (Contact under Companies), else derived.
//
// Steps 3 and 5 use only knownEntityNames and the URL, so an item of a type in
// package entities lands in the store Get, List and Count read for that type,
// unless step 1 or 2 matches first. The mock sees only the URL, so a type from
// outside this module on a URL that steps 3 or 5 claim for a type in package
// entities goes to that type's store (a type named Notes under Tickets/Notes
// gets TicketNotes) and needs a WithChildEntity route. On any other URL it is
// resolved by steps 2 and 4 and the derived fallback of step 5, which depend on
// the stores that exist. Lookup and creation share one lock hold, so concurrent creates
// under a new child URL share one store.
func (ts *TestServer) resolveChildStore(parent, segment string, create bool) (*entityStore, bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if name, ok := ts.childRoutes[parent+"/"+segment]; ok {
		if store, ok := ts.entities[name]; ok {
			return store, true
		}
	}
	derived := singularEntityName(parent) + segment
	if store, ok := ts.entities[derived]; ok {
		return store, true
	}
	name := derived
	if !knownEntityNames[derived] {
		if store, ok := ts.entities[segment]; ok {
			return store, true
		}
		if knownEntityNames[segment] {
			name = segment
		}
	}
	if !create {
		return nil, false
	}
	return ts.ensureStore(name), true
}

func (ts *TestServer) getStore(entityName string) (*entityStore, bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	store, ok := ts.entities[entityName]
	return store, ok
}

// storeFor returns the store a request path addresses, for a path that does
// not create one.
func (ts *TestServer) storeFor(p requestPath) (*entityStore, bool) {
	if p.child {
		return ts.resolveChildStore(p.parent, p.entity, false)
	}
	return ts.getStore(p.entity)
}

// writePage answers a list request with one page of items. A pageSize of 0 or
// less puts every item on one page. The page comes from the ?page=N query
// parameter, and a page past the last item answers an empty page.
func (ts *TestServer) writePage(w http.ResponseWriter, r *http.Request, items []json.RawMessage) {
	pageNum := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			pageNum = n
		}
	}
	pageSize := ts.opts.pageSize
	if pageSize <= 0 {
		pageSize = max(len(items), 1)
	}
	// Compare before multiplying so a huge ?page=N cannot overflow the offset.
	start := len(items)
	if pageNum-1 <= len(items)/pageSize {
		start = min((pageNum-1)*pageSize, len(items))
	}
	end := start + min(pageSize, len(items)-start)
	page := items[start:end]

	var nextPageURL any
	if end < len(items) {
		nextPageURL = fmt.Sprintf("%s%s?page=%d", ts.URL, r.URL.Path, pageNum+1)
	}
	var prevPageURL any
	if pageNum > 1 {
		prevPageURL = fmt.Sprintf("%s%s?page=%d", ts.URL, r.URL.Path, pageNum-1)
	}
	writeJSON(w, map[string]any{
		"items": page,
		"pageDetails": map[string]any{
			"count":        len(page),
			"requestCount": ts.opts.pageSize,
			"prevPageUrl":  prevPageURL,
			"nextPageUrl":  nextPageURL,
		},
	})
}

func (ts *TestServer) handleGet(w http.ResponseWriter, r *http.Request) {
	p := parsePath(r.URL.Path)
	store, ok := ts.storeFor(p)
	if !ok && p.child && p.id == 0 {
		// A child collection nothing was stored for is an empty list, as for a
		// parent without children. The slice is non-nil so it encodes as [].
		ts.writePage(w, r, []json.RawMessage{})
		return
	}
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, []string{fmt.Sprintf("Entity %q not found", p.entity)})
		return
	}

	if p.id == 0 {
		// Collection GET: a child list shows the items under its parent, a bare
		// entity GET shows every item.
		if p.child {
			ts.writePage(w, r, store.forParent(p.parentID))
		} else {
			ts.writePage(w, r, store.all())
		}
		return
	}

	var item json.RawMessage
	var found bool
	if p.child {
		item, found = store.getByIDForParent(p.id, p.parentID)
	} else {
		item, found = store.getByID(p.id)
	}
	if !found {
		// A missing id answers 200 with a null item, not 404, so tests exercise
		// the same client path as the real API.
		writeJSON(w, map[string]any{"item": nil})
		return
	}
	writeJSON(w, map[string]any{"item": item})
}

func (ts *TestServer) handleCreate(w http.ResponseWriter, r *http.Request) {
	p := parsePath(r.URL.Path)
	var store *entityStore
	if p.child {
		store, _ = ts.resolveChildStore(p.parent, p.entity, true)
	} else {
		// Auto-create store for unknown entities.
		ts.mu.Lock()
		store = ts.ensureStore(p.entity)
		ts.mu.Unlock()
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, []string{"Failed to read request body"})
		return
	}

	m, err := decodeObject(body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, []string{"Invalid JSON body"})
		return
	}

	// Validate required fields.
	for _, field := range store.requiredFields {
		if _, exists := m[field]; !exists {
			writeErrorResponse(w, http.StatusBadRequest, []string{fmt.Sprintf("Required field %q is missing", field)})
			return
		}
	}

	// Assign ID and store, recording the parent of a child create.
	newID := store.allocateID()
	m["id"] = newID
	data, err := json.Marshal(m)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, []string{"Failed to marshal entity"})
		return
	}
	store.add(data, p.parentID)

	writeJSON(w, map[string]any{"itemId": newID})
}

func (ts *TestServer) handleUpdate(w http.ResponseWriter, r *http.Request) {
	p := parsePath(r.URL.Path)
	store, ok := ts.storeFor(p)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, []string{fmt.Sprintf("Entity %q not found", p.entity)})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, []string{"Failed to read request body"})
		return
	}

	m, err := decodeObject(body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, []string{"Invalid JSON body"})
		return
	}

	id := extractID(m)
	if id == 0 {
		writeErrorResponse(w, http.StatusBadRequest, []string{"'id' field is required for update"})
		return
	}

	// Replace the stored entity so follow-up GETs return updated data. The item
	// keeps the parent it was created under.
	updated, err := json.Marshal(m)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, []string{"Failed to marshal updated entity"})
		return
	}
	var found bool
	if p.child {
		found = store.updateByIDForParent(id, p.parentID, updated)
	} else {
		found = store.updateByID(id, updated)
	}
	if !found {
		writeErrorResponse(w, http.StatusNotFound, []string{fmt.Sprintf("%s with ID %d not found", store.name, id)})
		return
	}

	// Return {"itemId": N} matching real Autotask PATCH behavior.
	writeJSON(w, map[string]any{"itemId": id})
}

func (ts *TestServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	p := parsePath(r.URL.Path)
	store, ok := ts.storeFor(p)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, []string{fmt.Sprintf("Entity %q not found", p.entity)})
		return
	}

	if !store.canDelete {
		writeErrorResponse(w, http.StatusBadRequest, []string{fmt.Sprintf("Entity %q does not support deletion", store.name)})
		return
	}

	var found bool
	if p.child {
		found = store.deleteByIDForParent(p.id, p.parentID)
	} else {
		found = store.deleteByID(p.id)
	}
	if !found {
		writeErrorResponse(w, http.StatusNotFound, []string{fmt.Sprintf("%s with ID %d not found", store.name, p.id)})
		return
	}

	writeJSON(w, map[string]any{"itemId": p.id})
}

// Metadata handlers.

func (ts *TestServer) handleEntityInfo(w http.ResponseWriter, r *http.Request) {
	// Extract entity name from path like /v1.0/Companies/entityInformation
	trimmed := strings.TrimPrefix(r.URL.Path, "/v1.0/")
	parts := strings.SplitN(trimmed, "/", 2) //nolint:mnd // split into at most 2 parts
	entityName := parts[0]

	md, ok := ts.opts.metadata[entityName]
	if !ok || md.info == nil {
		writeJSON(w, map[string]any{"info": EntityInfoResponse{Name: entityName, CanCreate: true, CanUpdate: true, CanQuery: true}})
		return
	}
	writeJSON(w, map[string]any{"info": md.info})
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
	// The wire key is "apiVersions" (the key discoverZone decodes), not "versions".
	// Advertise the realistic list Autotask currently returns.
	writeJSON(w, map[string]any{"apiVersions": []string{"V1.0", "V2.0"}})
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

// decodeObject decodes a JSON object with numbers kept as json.Number, so an
// id above 2^53 survives create, Get, Update and Delete by id exactly. Query
// filters still compare numbers as float64. A null body or data after the
// object is an error.
func decodeObject(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("autotasktest: body is not a JSON object")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("autotasktest: data after the JSON object")
	}
	return m, nil
}
