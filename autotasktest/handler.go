package autotasktest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
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

// owner returns the owner an item created at, or visible under, this path has.
// A root path has no parent, so its owner is the zero owner.
func (p requestPath) owner() owner { return owner{parent: p.parent, id: p.parentID} }

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
// under a new child URL share one store. The write gates of WithChildOnlyWrites,
// WithoutCreateSupport, WithoutUpdateSupport and WithDeleteSupport apply to the
// store this returns.
func (ts *TestServer) resolveChildStore(parent, segment string, create bool) (*entityStore, bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	name, found := ts.childStoreNameLocked(parent, segment)
	if found {
		return ts.entities[name], true
	}
	if !create {
		return nil, false
	}
	return ts.ensureStore(name), true
}

// childStoreNameLocked applies the resolution order of resolveChildStore and
// returns the store name it picks. found is true when that store exists; when it
// is false, name is the store a create makes (step 5). It creates nothing. The
// caller holds ts.mu.
func (ts *TestServer) childStoreNameLocked(parent, segment string) (name string, found bool) {
	if name, ok := ts.childRoutes[parent+"/"+segment]; ok {
		if _, ok := ts.entities[name]; ok {
			return name, true
		}
	}
	derived := singularEntityName(parent) + segment
	if _, ok := ts.entities[derived]; ok {
		return derived, true
	}
	if !knownEntityNames[derived] {
		if _, ok := ts.entities[segment]; ok {
			return segment, true
		}
	}
	return createdChildName(derived, segment), false
}

// createdChildName returns the name of the store a create makes for a child URL
// that matched no store (step 5 of resolveChildStore): the segment when that is
// the entity name of a type in package entities and derived is not, else
// derived.
func createdChildName(derived, segment string) string {
	if !knownEntityNames[derived] && knownEntityNames[segment] {
		return segment
	}
	return derived
}

// writeOp is the kind of a write request.
type writeOp int

const (
	opCreate writeOp = iota
	opUpdate
	opDelete
)

// formatRoutes lists "Parent/Segment" routes as /v1.0/{Parent}/{id}/{Segment}
// URLs, sorted and joined with " or ".
func formatRoutes(routes map[string]bool) string {
	urls := make([]string, 0, len(routes))
	for _, route := range slices.Sorted(maps.Keys(routes)) {
		parent, segment, _ := strings.Cut(route, "/")
		urls = append(urls, fmt.Sprintf("/v1.0/%s/{id}/%s", parent, segment))
	}
	return strings.Join(urls, " or ")
}

// onAllowedRoute reports whether the request is a child request on one of the
// store's writeRoutes.
func (s *entityStore) onAllowedRoute(p requestPath) bool {
	return p.child && s.writeRoutes[p.parent+"/"+p.entity]
}

// writeRejection returns why the store refuses the write, or "" when it
// accepts it. The checks read flags that the options set before the server
// starts and nothing changes afterwards.
func (s *entityStore) writeRejection(op writeOp, p requestPath) string {
	switch {
	case len(s.writeRoutes) > 0 && !s.onAllowedRoute(p):
		return fmt.Sprintf("Entity %q is written only through %s", s.name, formatRoutes(s.writeRoutes))
	case op == opCreate && s.noCreate:
		return fmt.Sprintf("Entity %q does not support creation", s.name)
	case op == opUpdate && s.noUpdate:
		return fmt.Sprintf("Entity %q does not support update", s.name)
	case op == opDelete && !s.canDelete:
		return fmt.Sprintf("Entity %q does not support deletion", s.name)
	}
	return ""
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

// itemsAt returns the items the collection at path p shows: for a child path
// the items visible under its parent (see forParent), for a root path every
// item. The child list, query and count all read through it.
func (s *entityStore) itemsAt(p requestPath) []json.RawMessage {
	if p.child {
		return s.forParent(p.owner())
	}
	return s.all()
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
		ts.writePage(w, r, store.itemsAt(p))
		return
	}

	var item json.RawMessage
	var found bool
	if p.child {
		item, found = store.getByIDForParent(p.id, p.owner())
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

// createStoreFor returns the store a create request writes to, making it when
// absent. It returns a rejection and no store when the request is a child
// create under a segment that a WithChildOnlyWrites type uses, matches no
// store, and would make a store that is not an entity type of this module.
func (ts *TestServer) createStoreFor(p requestPath) (store *entityStore, rejection string) {
	if !p.child {
		// Auto-create store for unknown entities.
		ts.mu.Lock()
		defer ts.mu.Unlock()
		return ts.ensureStore(p.entity), ""
	}
	if routes := ts.childOnlySegments[p.entity]; len(routes) > 0 {
		derived := singularEntityName(p.parent) + p.entity
		if _, found := ts.storeFor(p); !found && !knownEntityNames[createdChildName(derived, p.entity)] {
			return nil, fmt.Sprintf("Child segment %q under %q is written only through %s", p.entity, p.parent, formatRoutes(routes))
		}
	}
	store, _ = ts.resolveChildStore(p.parent, p.entity, true)
	return store, ""
}

func (ts *TestServer) handleCreate(w http.ResponseWriter, r *http.Request) {
	p := parsePath(r.URL.Path)
	store, rejection := ts.createStoreFor(p)
	if rejection == "" {
		rejection = store.writeRejection(opCreate, p)
	}
	if rejection != "" {
		writeErrorResponse(w, http.StatusBadRequest, []string{rejection})
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
	store.add(data, p.owner())

	writeJSON(w, map[string]any{"itemId": newID})
}

func (ts *TestServer) handleUpdate(w http.ResponseWriter, r *http.Request) {
	p := parsePath(r.URL.Path)
	store, ok := ts.storeFor(p)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, []string{fmt.Sprintf("Entity %q not found", p.entity)})
		return
	}
	if rejection := store.writeRejection(opUpdate, p); rejection != "" {
		writeErrorResponse(w, http.StatusBadRequest, []string{rejection})
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
		found = store.updateByIDForParent(id, p.owner(), updated)
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

	if rejection := store.writeRejection(opDelete, p); rejection != "" {
		writeErrorResponse(w, http.StatusBadRequest, []string{rejection})
		return
	}

	var found bool
	if p.child {
		found = store.deleteByIDForParent(p.id, p.owner())
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

// Suffixes of the metadata request paths. route dispatches on them and
// metadataEntity strips them.
const (
	entityInfoSuffix   = "/entityInformation"
	entityFieldsSuffix = "/entityInformation/fields"
	entityUDFsSuffix   = "/entityInformation/userDefinedFields"
)

// metadataEntity returns the entity name a metadata request path asks about.
// A root path /v1.0/{entity}{suffix} names the entity itself. A child path
// /v1.0/{parent}/{id}/{segment}{suffix} names the entity whose store that child
// URL resolves to, whether or not the store exists yet, and creates none.
func (ts *TestServer) metadataEntity(path, suffix string) string {
	p := parsePath(strings.TrimSuffix(path, suffix))
	if !p.child {
		return p.entity
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	name, _ := ts.childStoreNameLocked(p.parent, p.entity)
	return name
}

func (ts *TestServer) handleEntityInfo(w http.ResponseWriter, r *http.Request) {
	entityName := ts.metadataEntity(r.URL.Path, entityInfoSuffix)

	md, ok := ts.opts.metadata[entityName]
	if !ok || md.info == nil {
		info := EntityInfoResponse{Name: entityName, CanCreate: true, CanUpdate: true, CanQuery: true}
		if store, found := ts.getStore(entityName); found {
			info.CanCreate = !store.noCreate
			info.CanUpdate = !store.noUpdate
			info.CanDelete = store.canDelete
		}
		writeJSON(w, map[string]any{"info": info})
		return
	}
	writeJSON(w, map[string]any{"info": md.info})
}

func (ts *TestServer) handleEntityFields(w http.ResponseWriter, r *http.Request) {
	entityName := ts.metadataEntity(r.URL.Path, entityFieldsSuffix)

	md, ok := ts.opts.metadata[entityName]
	if !ok || md.fields == nil {
		writeJSON(w, map[string]any{"fields": []any{}})
		return
	}
	writeJSON(w, map[string]any{"fields": md.fields})
}

func (ts *TestServer) handleEntityUDFs(w http.ResponseWriter, r *http.Request) {
	entityName := ts.metadataEntity(r.URL.Path, entityUDFsSuffix)

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
// id above 2^53 is not rounded through float64. A body that is not a JSON
// object, a null body, or data after the object is an error.
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
