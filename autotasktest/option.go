package autotasktest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// ServerOption configures a TestServer.
type ServerOption func(*TestServer)

// storedItem is one stored entity and the parent it was created under.
type storedItem struct {
	data json.RawMessage // raw JSON for flexible matching
	// parentID is the id in the child create URL, or 0 for an item seeded with
	// WithEntity or created through the root URL. Such an item is listed under
	// every parent.
	parentID int64
}

// underParent reports whether the item is visible under the URL parent pid.
func (it storedItem) underParent(pid int64) bool {
	return it.parentID == 0 || it.parentID == pid
}

// entityStore holds in-memory entities for a single entity type.
type entityStore struct {
	mu             sync.RWMutex
	name           string
	items          []storedItem
	idField        string   // JSON field name for the ID (always "id")
	requiredFields []string // fields required on create
	canDelete      bool
	nextID         int64
}

func newEntityStore(name string) *entityStore {
	return &entityStore{
		name:    name,
		idField: "id",
		nextID:  1,
	}
}

// add stores data under parentID (0 for no parent).
func (s *entityStore) add(data json.RawMessage, parentID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, storedItem{data: data, parentID: parentID})
}

// indexOf returns the position of the item with the given id for which match
// holds, or -1. The caller holds s.mu.
func (s *entityStore) indexOf(id int64, match func(storedItem) bool) int {
	for i, item := range s.items {
		var m map[string]any
		if err := json.Unmarshal(item.data, &m); err != nil {
			continue
		}
		if extractID(m) == id && match(item) {
			return i
		}
	}
	return -1
}

func anyParent(storedItem) bool { return true }

func parentMatcher(pid int64) func(storedItem) bool {
	return func(it storedItem) bool { return it.underParent(pid) }
}

func (s *entityStore) getByID(id int64) (json.RawMessage, bool) {
	return s.getMatching(id, anyParent)
}

// getByIDForParent returns the item only when it is visible under parent pid.
func (s *entityStore) getByIDForParent(id, pid int64) (json.RawMessage, bool) {
	return s.getMatching(id, parentMatcher(pid))
}

func (s *entityStore) getMatching(id int64, match func(storedItem) bool) (json.RawMessage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if i := s.indexOf(id, match); i >= 0 {
		return s.items[i].data, true
	}
	return nil, false
}

// updateByID replaces the item's data and keeps its stored parent id: a PATCH
// body carries no parent.
func (s *entityStore) updateByID(id int64, data json.RawMessage) bool {
	return s.updateMatching(id, data, anyParent)
}

// updateByIDForParent is updateByID limited to items visible under parent pid.
func (s *entityStore) updateByIDForParent(id, pid int64, data json.RawMessage) bool {
	return s.updateMatching(id, data, parentMatcher(pid))
}

func (s *entityStore) updateMatching(id int64, data json.RawMessage, match func(storedItem) bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexOf(id, match)
	if i < 0 {
		return false
	}
	s.items[i].data = data
	return true
}

func (s *entityStore) deleteByID(id int64) bool {
	return s.deleteMatching(id, anyParent)
}

// deleteByIDForParent is deleteByID limited to items visible under parent pid.
func (s *entityStore) deleteByIDForParent(id, pid int64) bool {
	return s.deleteMatching(id, parentMatcher(pid))
}

func (s *entityStore) deleteMatching(id int64, match func(storedItem) bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexOf(id, match)
	if i < 0 {
		return false
	}
	s.items = append(s.items[:i], s.items[i+1:]...)
	return true
}

// all returns every item, whatever its parent.
func (s *entityStore) all() []json.RawMessage {
	return s.collect(anyParent)
}

// forParent returns the items visible under parent pid: those created under pid
// and those with no parent. The slice is never nil, so it encodes as [].
func (s *entityStore) forParent(pid int64) []json.RawMessage {
	return s.collect(parentMatcher(pid))
}

func (s *entityStore) collect(match func(storedItem) bool) []json.RawMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]json.RawMessage, 0, len(s.items))
	for _, it := range s.items {
		if match(it) {
			out = append(out, it.data)
		}
	}
	return out
}

func (s *entityStore) allocateID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID
	s.nextID++
	return id
}

// extractID pulls the numeric ID from a JSON object map.
func extractID(m map[string]any) int64 {
	v, ok := m["id"]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

// ensureStore returns the store named name, creating it when absent. It is for
// option functions, which run before the server starts.
func (ts *TestServer) ensureStore(name string) *entityStore {
	store, ok := ts.entities[name]
	if !ok {
		store = newEntityStore(name)
		ts.entities[name] = store
	}
	return store
}

// WithEntity seeds the server with one or more entities of type T.
// The entity's EntityName() determines which store it goes into. The items have
// no parent, so a child list under any parent includes them. A child URL finds
// this store through the resolution rule described on WithChildEntity, so no
// child segment alias is registered.
func WithEntity[T autotask.Entity](items ...T) ServerOption {
	return func(ts *TestServer) {
		if len(items) == 0 {
			return
		}
		name := items[0].EntityName()
		store := ts.ensureStore(name)
		for _, item := range items {
			data, err := json.Marshal(item)
			if err != nil {
				ts.t.Fatalf("autotasktest: marshaling %s fixture: %v", name, err)
			}
			store.add(data, 0)
		}
	}
}

// WithChildEntity seeds child entities of type C under the parent entity of
// type P with the given id. The items go into the store named C.EntityName(),
// the same store Get, List and Count read, and a child list under parentID
// includes them while a list under another parent does not.
//
// A child URL /v1.0/{Parent}/{id}/{Segment} resolves to a store in this order:
// the route a WithChildEntity registered for that parent and segment, the store
// named singular(Parent)+Segment (Tickets and Attachments give
// TicketAttachments), then the store named Segment. A create under an unknown
// child URL makes the store singular(Parent)+Segment. WithChildEntity is needed
// to seed children under a parent, and for a child type whose entity name is
// neither of those two names. Registering the same parent and segment for two
// different entity types fails the test.
func WithChildEntity[P autotask.Entity, C autotask.Entity](parentID int64, items ...C) ServerOption {
	return func(ts *TestServer) {
		var parent P
		var child C
		name := child.EntityName()
		segment := name
		if ce, ok := any(child).(autotask.ChildEntity); ok {
			segment = ce.ChildEntityName()
		}
		route := parent.EntityName() + "/" + segment
		if prev, ok := ts.childRoutes[route]; ok && prev != name {
			ts.t.Fatalf("autotasktest: child route %s is already registered for %s, cannot register it for %s", route, prev, name)
			return
		}
		ts.childRoutes[route] = name
		store := ts.ensureStore(name)
		for _, item := range items {
			data, err := json.Marshal(item)
			if err != nil {
				ts.t.Fatalf("autotasktest: marshaling %s fixture: %v", name, err)
			}
			store.add(data, parentID)
		}
	}
}

// WithAuth sets the expected authentication credentials.
func WithAuth(username, secret, integrationCode string) ServerOption {
	return func(ts *TestServer) {
		ts.auth = authConfig{
			username:        username,
			secret:          secret,
			integrationCode: integrationCode,
		}
	}
}

// WithPageSize sets the maximum number of items per page for query responses.
func WithPageSize(n int) ServerOption {
	return func(ts *TestServer) {
		ts.opts.pageSize = n
	}
}

// RuleOption adjusts a rule created by WithErrorOn, WithResponseOn or
// WithRetryAfterError.
type RuleOption func(*errorRule)

// Times limits a rule to the first n matching requests. Later requests fall
// through to the next matching rule, or to the normal handler. n <= 0 leaves
// the rule unlimited, which is the default.
func Times(n int) RuleOption {
	return func(r *errorRule) {
		r.limit = max(n, 0)
	}
}

func (ts *TestServer) addRule(rule *errorRule, opts []RuleOption) {
	for _, opt := range opts {
		opt(rule)
	}
	ts.opts.errorRules = append(ts.opts.errorRules, *rule)
}

// WithErrorOn injects an error response for requests matching method + path suffix.
// An empty method matches every method. Rules are tried in registration order
// and the first one that matches and has requests left answers.
func WithErrorOn(method, pathSuffix string, status int, errors []string, opts ...RuleOption) ServerOption {
	return func(ts *TestServer) {
		ts.addRule(&errorRule{
			method:     method,
			pathSuffix: pathSuffix,
			status:     status,
			errors:     errors,
		}, opts)
	}
}

// WithResponseOn serves a fixed response for requests matching method + path
// suffix. The body is sent as is, so it can be HTML, empty or invalid JSON,
// such as the maintenance page Autotask serves with HTTP 200. An empty
// contentType sends no Content-Type header. Rules run before the auth check
// and follow the ordering and Times semantics of WithErrorOn.
func WithResponseOn(method, pathSuffix string, status int, contentType, body string, opts ...RuleOption) ServerOption {
	return func(ts *TestServer) {
		ts.addRule(&errorRule{
			method:     method,
			pathSuffix: pathSuffix,
			status:     status,
			raw:        &rawResponse{contentType: contentType, body: body},
		}, opts)
	}
}

// WithServerLatency adds artificial latency to all responses.
func WithServerLatency(d time.Duration) ServerOption {
	return func(ts *TestServer) {
		ts.opts.latency = d
	}
}

// WithEntityMetadata configures metadata responses for an entity.
func WithEntityMetadata(entityName string, info EntityInfoResponse, fields []FieldInfoResponse, udfs []UDFInfoResponse) ServerOption {
	return func(ts *TestServer) {
		ts.opts.metadata[entityName] = &entityMetadata{
			info:   &info,
			fields: fields,
			udfs:   udfs,
		}
	}
}

// WithZoneInfo configures the zone discovery response.
func WithZoneInfo(name, url string) ServerOption {
	return func(ts *TestServer) {
		ts.opts.zoneInfo = &zoneConfig{name: name, url: url}
	}
}

// WithRequiredFields sets the required fields for entity creation validation.
func WithRequiredFields(entityName string, fields ...string) ServerOption {
	return func(ts *TestServer) {
		store := ts.ensureStore(entityName)
		store.requiredFields = fields
	}
}

// WithDeleteSupport marks an entity type as supporting DELETE.
func WithDeleteSupport(entityName string) ServerOption {
	return func(ts *TestServer) {
		store := ts.ensureStore(entityName)
		store.canDelete = true
	}
}

// WithRetryAfterError injects a 429 response with Retry-After header for a path.
func WithRetryAfterError(pathSuffix string, retryAfterSeconds int, opts ...RuleOption) ServerOption {
	return func(ts *TestServer) {
		ts.addRule(&errorRule{
			method:     "",
			pathSuffix: pathSuffix,
			status:     http.StatusTooManyRequests,
			errors:     []string{fmt.Sprintf("Rate limit exceeded. Retry after %d seconds.", retryAfterSeconds)},
			headers:    map[string]string{"Retry-After": strconv.Itoa(retryAfterSeconds)},
		}, opts)
	}
}
