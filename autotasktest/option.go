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

// entityStore holds in-memory entities for a single entity type.
type entityStore struct {
	mu             sync.RWMutex
	name           string
	items          []json.RawMessage // stored as raw JSON for flexible matching
	idField        string            // JSON field name for the ID (always "id")
	requiredFields []string          // fields required on create
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

func (s *entityStore) add(data json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, data)
}

func (s *entityStore) getByID(id int64) (json.RawMessage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		var m map[string]any
		if err := json.Unmarshal(item, &m); err != nil {
			continue
		}
		if extractID(m) == id {
			return item, true
		}
	}
	return nil, false
}

func (s *entityStore) updateByID(id int64, data json.RawMessage) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.items {
		var m map[string]any
		if err := json.Unmarshal(item, &m); err != nil {
			continue
		}
		if extractID(m) == id {
			s.items[i] = data
			return true
		}
	}
	return false
}

func (s *entityStore) deleteByID(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.items {
		var m map[string]any
		if err := json.Unmarshal(item, &m); err != nil {
			continue
		}
		if extractID(m) == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return true
		}
	}
	return false
}

func (s *entityStore) all() []json.RawMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]json.RawMessage, len(s.items))
	copy(cp, s.items)
	return cp
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

// WithEntity seeds the server with one or more entities of type T.
// The entity's EntityName() determines which store it goes into.
// If the entity implements ChildEntity, the store is also registered under
// the child name so child URL lookups resolve correctly.
func WithEntity[T autotask.Entity](items ...T) ServerOption {
	return func(ts *TestServer) {
		if len(items) == 0 {
			return
		}
		name := items[0].EntityName()
		store, ok := ts.entities[name]
		if !ok {
			store = newEntityStore(name)
			ts.entities[name] = store
		}
		// Also register under ChildEntityName so child URL lookups work.
		if ce, ok := any(items[0]).(autotask.ChildEntity); ok {
			childName := ce.ChildEntityName()
			if childName != name {
				ts.entities[childName] = store
			}
		}
		for _, item := range items {
			data, err := json.Marshal(item)
			if err != nil {
				ts.t.Fatalf("autotasktest: marshaling %s fixture: %v", name, err)
			}
			store.add(data)
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

// WithErrorOn injects an error response for requests matching method + path suffix.
func WithErrorOn(method, pathSuffix string, status int, errors []string) ServerOption {
	return func(ts *TestServer) {
		ts.opts.errorRules = append(ts.opts.errorRules, errorRule{
			method:     method,
			pathSuffix: pathSuffix,
			status:     status,
			errors:     errors,
		})
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
		store, ok := ts.entities[entityName]
		if !ok {
			store = newEntityStore(entityName)
			ts.entities[entityName] = store
		}
		store.requiredFields = fields
	}
}

// WithDeleteSupport marks an entity type as supporting DELETE.
func WithDeleteSupport(entityName string) ServerOption {
	return func(ts *TestServer) {
		store, ok := ts.entities[entityName]
		if !ok {
			store = newEntityStore(entityName)
			ts.entities[entityName] = store
		}
		store.canDelete = true
	}
}

// WithRetryAfterError injects a 429 response with Retry-After header for a path.
func WithRetryAfterError(pathSuffix string, retryAfterSeconds int) ServerOption {
	return func(ts *TestServer) {
		ts.opts.errorRules = append(ts.opts.errorRules, errorRule{
			method:     "",
			pathSuffix: pathSuffix,
			status:     http.StatusTooManyRequests,
			errors:     []string{fmt.Sprintf("Rate limit exceeded. Retry after %d seconds.", retryAfterSeconds)},
			headers:    map[string]string{"Retry-After": strconv.Itoa(retryAfterSeconds)},
		})
	}
}
