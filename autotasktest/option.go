package autotasktest

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// ServerOption configures a TestServer.
type ServerOption func(*TestServer)

// owner is the parent collection segment and parent id of the child URL an item
// was created at, or of the route WithChildEntity registered. The zero owner
// means no parent: an item seeded with WithEntity, created through the root URL,
// or given parent id 0.
type owner struct {
	parent string
	id     int64
}

// storedItem is one stored entity and the owner it was created under.
type storedItem struct {
	data json.RawMessage // raw JSON for flexible matching
	// owner is the zero owner for an item with no parent. Such an item is
	// visible under every parent.
	owner owner
}

// underParent reports whether the item is visible under the URL parent o: it
// has no owner, or its owner is o.
func (it storedItem) underParent(o owner) bool {
	return it.owner == (owner{}) || it.owner == o
}

// entityStore holds in-memory entities for a single entity type.
type entityStore struct {
	mu             sync.RWMutex
	name           string
	items          []storedItem
	idField        string   // JSON field name for the ID (always "id")
	requiredFields []string // fields required on create
	canDelete      bool     // set by WithDeleteSupport
	noCreate       bool     // set by WithoutCreateSupport
	noUpdate       bool     // set by WithoutUpdateSupport
	// writeRoutes holds the "Parent/Segment" child routes that WithChildOnlyWrites
	// allows writes on. A nil map leaves the store unrestricted.
	writeRoutes map[string]bool
	nextID      int64
}

func newEntityStore(name string) *entityStore {
	return &entityStore{
		name:    name,
		idField: "id",
		nextID:  1,
	}
}

// add stores data under its owner, the zero owner for no parent (an owner with
// id 0 counts as no parent). It moves the id allocator past the item's id, so a
// create after a seeded item never reuses its id.
func (s *entityStore) add(data json.RawMessage, o owner) {
	if o.id == 0 {
		o = owner{}
	}
	var ref struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(data, &ref) // an item without a numeric id reserves nothing
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, storedItem{data: data, owner: o})
	if ref.ID >= s.nextID && ref.ID < math.MaxInt64 {
		s.nextID = ref.ID + 1
	}
}

// indexOf returns the position of the item with the given id for which match
// holds, or -1. The caller holds s.mu.
func (s *entityStore) indexOf(id int64, match func(storedItem) bool) int {
	for i, item := range s.items {
		m, err := decodeObject(item.data)
		if err != nil {
			continue
		}
		if extractID(m) == id && match(item) {
			return i
		}
	}
	return -1
}

func anyParent(storedItem) bool { return true }

func parentMatcher(o owner) func(storedItem) bool {
	return func(it storedItem) bool { return it.underParent(o) }
}

func (s *entityStore) getByID(id int64) (json.RawMessage, bool) {
	return s.getMatching(id, anyParent)
}

// getByIDForParent returns the item only when it is visible under the parent o.
func (s *entityStore) getByIDForParent(id int64, o owner) (json.RawMessage, bool) {
	return s.getMatching(id, parentMatcher(o))
}

func (s *entityStore) getMatching(id int64, match func(storedItem) bool) (json.RawMessage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if i := s.indexOf(id, match); i >= 0 {
		return s.items[i].data, true
	}
	return nil, false
}

// updateByID replaces the item's data and keeps its stored owner: a PATCH body
// carries no parent.
func (s *entityStore) updateByID(id int64, data json.RawMessage) bool {
	return s.updateMatching(id, data, anyParent)
}

// updateByIDForParent is updateByID limited to items visible under the parent o.
func (s *entityStore) updateByIDForParent(id int64, o owner, data json.RawMessage) bool {
	return s.updateMatching(id, data, parentMatcher(o))
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

// deleteByIDForParent is deleteByID limited to items visible under the parent o.
func (s *entityStore) deleteByIDForParent(id int64, o owner) bool {
	return s.deleteMatching(id, parentMatcher(o))
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

// forParent returns the items visible under the parent o: those created under o
// and those with no parent. The slice is never nil, so it encodes as [].
func (s *entityStore) forParent(o owner) []json.RawMessage {
	return s.collect(parentMatcher(o))
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

// ensureStore returns the store named name, creating it when absent. Option
// functions call it before the server starts; a request handler must hold ts.mu.
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
			store.add(data, owner{})
		}
	}
}

// childRoute returns the entity name of C, its child URL segment, and the
// "Parent/Segment" route the typed child calls send for C under P. The segment
// is C's ChildEntityName, else its entity name. The parent half is C's
// ChildParentName, else P's entity name.
func childRoute[P, C autotask.Entity]() (name, segment, route string) {
	var parent P
	var child C
	name = child.EntityName()
	segment = name
	if ce, ok := any(child).(autotask.ChildEntity); ok {
		segment = ce.ChildEntityName()
	}
	parentSegment := parent.EntityName()
	if cp, ok := any(child).(autotask.ChildParentEntity); ok {
		parentSegment = cp.ChildParentName()
	}
	return name, segment, parentSegment + "/" + segment
}

// registerChildRoute maps route to the store name. It fails the test and
// returns false when route is already registered for another entity.
func (ts *TestServer) registerChildRoute(route, name string) bool {
	if prev, ok := ts.childRoutes[route]; ok && prev != name {
		ts.t.Fatalf("autotasktest: child route %s is already registered for %s, cannot register it for %s", route, prev, name)
		return false
	}
	ts.childRoutes[route] = name
	return true
}

// WithChildEntity seeds child entities of type C under the parent entity of
// type P with the given id. The items go into the store named C.EntityName(),
// the same store Get, List and Count read. A child list at the route's parent
// collection and parentID includes them, and a list under another parent id, or
// under another parent collection that resolves to the same store, does not.
//
// A child URL /v1.0/{Parent}/{id}/{Segment} resolves to a store in this order:
// the route a WithChildEntity registered for that parent and segment, else the
// store named singular(Parent)+Segment (Tickets and Attachments give
// TicketAttachments). For a type in package entities, that name or the segment
// is then chosen by entity name alone: when singular(Parent)+Segment is the
// entity name of such a type, a create makes that store, and the store named
// Segment is not used; when only Segment is (Contact under Companies, Task under
// Projects), a create makes the store named Segment. So these types read back
// through Get, List and Count, unless a store named singular(Parent)+Segment
// already exists. The mock sees only the URL, so a type from outside this module
// on a URL that names a type in package entities this way goes to that type's
// store, and needs WithChildEntity. On any other URL it resolves to an existing
// store named Segment if there is one, else a create makes
// singular(Parent)+Segment. Such a type whose entity name is the segment reads
// back once its store exists (a WithEntity seed creates it); one whose entity
// name is neither name needs WithChildEntity. WithChildEntity is also needed to
// seed children under a parent. Registering the same parent
// and segment for two different entity types fails the test. When C implements
// autotask.ChildParentEntity, the route's parent half is its ChildParentName,
// the segment the typed child calls send, instead of P's entity name.
func WithChildEntity[P autotask.Entity, C autotask.Entity](parentID int64, items ...C) ServerOption {
	return func(ts *TestServer) {
		name, _, route := childRoute[P, C]()
		if !ts.registerChildRoute(route, name) {
			return
		}
		store := ts.ensureStore(name)
		parent, _, _ := strings.Cut(route, "/")
		for _, item := range items {
			data, err := json.Marshal(item)
			if err != nil {
				ts.t.Fatalf("autotasktest: marshaling %s fixture: %v", name, err)
			}
			store.add(data, owner{parent: parent, id: parentID})
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

// WithPageSize sets the maximum number of items per page for query and list
// responses. A size of 0 or less puts every item on one page.
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

// WithEntityMetadata configures metadata responses for an entity. The info
// replaces the default entityInformation answer, including the flags that
// WithDeleteSupport, WithoutCreateSupport and WithoutUpdateSupport set.
//
// A metadata path under a child URL (/v1.0/{Parent}/{id}/{Segment}/entityInformation
// and its /fields and /userDefinedFields) answers for the entity that child URL
// resolves to, as described on WithChildEntity, so entityName is that entity's
// name (for example TicketAdditionalConfigurationItems). Such a request creates
// no store.
func WithEntityMetadata(entityName string, info EntityInfoResponse, fields []FieldInfoResponse, udfs []UDFInfoResponse) ServerOption {
	return func(ts *TestServer) {
		ts.opts.metadata[entityName] = &entityMetadata{
			info:   &info,
			fields: fields,
			udfs:   udfs,
		}
	}
}

// WithZoneInfo configures the zone discovery response. A client that discovers
// against the server rejects a url that is not an absolute http or https URL
// with a host and no "@", query or fragment.
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

// WithDeleteSupport marks an entity type as supporting DELETE, and makes the
// default entityInformation answer report canDelete true.
func WithDeleteSupport(entityName string) ServerOption {
	return func(ts *TestServer) {
		store := ts.ensureStore(entityName)
		store.canDelete = true
	}
}

// WithChildOnlyWrites makes the mock accept writes of C only on the child URL
// that CreateChild[P], UpdateChild[P] and DeleteChild[P, C] send. The route is
// computed as WithChildEntity computes it, so it follows C's ChildEntityName
// and ChildParentName, and a conflicting route fails the test. Call it again
// with another P to allow more than one parent.
//
// A create, update or delete of C at the root URL answers 400, and so does one
// at a child URL that resolves to C's store under another parent or segment
// (for example /Quotes/{id}/QuoteItems for QuoteItem). A create under C's
// segment that matches no store and no type in package entities answers 400
// too (for example /ExpenseReports/{id}/Items for ExpenseItem), and so does a
// create under that segment for a type from outside this module that has no
// WithChildEntity route. A segment no marked type uses keeps the junk-store
// fallback described on WithChildEntity.
//
// Get, List, Count and queries are not affected, and an item can still be
// seeded. Delete still needs WithDeleteSupport. The only allowed routes are the
// ones given to this option: a WithChildEntity seed under another parent still
// serves reads there. A child URL that resolves to another store keeps the
// resolution described on WithChildEntity.
//
// The 400 mirrors the answer for an entity without delete support. What the
// live API answers for these URLs has not been verified, so a test should
// assert that the write fails rather than the error type.
func WithChildOnlyWrites[P, C autotask.Entity]() ServerOption {
	return func(ts *TestServer) {
		name, segment, route := childRoute[P, C]()
		if !ts.registerChildRoute(route, name) {
			return
		}
		store := ts.ensureStore(name)
		if store.writeRoutes == nil {
			store.writeRoutes = make(map[string]bool)
		}
		store.writeRoutes[route] = true
		if ts.childOnlySegments == nil {
			ts.childOnlySegments = make(map[string]map[string]bool)
		}
		if ts.childOnlySegments[segment] == nil {
			ts.childOnlySegments[segment] = make(map[string]bool)
		}
		ts.childOnlySegments[segment][route] = true
	}
}

// WithoutUpdateSupport makes PATCH for an entity answer 400 at the root URL
// and at every child URL that resolves to its store, and the default
// entityInformation answer report canUpdate false. Use it for entities the API cannot update, such as
// ChangeRequestLinks. WithEntityMetadata replaces the default answer.
func WithoutUpdateSupport(entityName string) ServerOption {
	return func(ts *TestServer) {
		ts.ensureStore(entityName).noUpdate = true
	}
}

// WithoutCreateSupport makes POST for an entity answer 400 at the root URL
// and at every child URL that resolves to its store, and the default
// entityInformation answer report canCreate false.
// Use it for entities the API cannot create, such as Resources.
// WithEntityMetadata replaces the default answer.
func WithoutCreateSupport(entityName string) ServerOption {
	return func(ts *TestServer) {
		ts.ensureStore(entityName).noCreate = true
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
