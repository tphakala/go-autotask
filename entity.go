package autotask

import (
	"encoding/json"
	"fmt"
)

// Entity is the interface all typed Autotask entities implement.
// EntityName() MUST use a value receiver to prevent double-pointer issues
// with generic CRUD functions (e.g., Get[*Ticket] would return **Ticket).
type Entity interface {
	EntityName() string
}

// UDF represents a user-defined field value.
type UDF struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// ChildEntity is an optional interface for entities accessed as children of
// another entity. The Autotask REST API uses a shorter URL segment for child
// access than for direct CRUD. For example, TicketNote uses "TicketNotes" for
// direct CRUD (/v1.0/TicketNotes/{id}) but "Notes" as a child of Tickets
// (/v1.0/Tickets/{parentId}/Notes).
type ChildEntity interface {
	ChildEntityName() string
}

// ChildParentEntity is an optional interface for child entities whose child
// URLs name the parent collection with a segment other than the parent's
// EntityName. ExpenseItem is reached through /v1.0/Expenses/{id}/Items, while
// its parent ExpenseReport has the EntityName "ExpenseReports", which the root
// URLs use. The typed child calls (GetChild, ListChild, ListChildIter,
// CreateChild, UpdateChild and DeleteChild) use ChildParentName in place of the
// parent type's EntityName, whichever parent type P is passed, as
// ChildEntityName applies for any P.
type ChildParentEntity interface {
	ChildParentName() string
}

// EntityWithID is an optional interface that entities can implement
// to receive the server-generated ID from Create operations.
// The Autotask API returns {"itemId": N} on successful creates.
//
// Entities that do not implement it still receive the id: Create and
// CreateChild decode {"id": N} into the entity, which sets the member whose
// JSON name is "id" (matched without regard to case; every type in the
// entities package uses exactly "id"). For a type without its own
// UnmarshalJSON no other member changes. Implement EntityWithID for a type
// whose id member has a different JSON name or type, or that has its own
// UnmarshalJSON.
type EntityWithID interface {
	SetID(id int64)
}

// assignCreatedID stores the server-assigned id on a created entity. entity
// must be a non-nil pointer. A decode error is ignored: the record already
// exists on the server, so Create must not report an error that invites a
// duplicate.
func assignCreatedID(entity any, id int64) {
	if setter, ok := entity.(EntityWithID); ok {
		setter.SetID(id)
		return
	}
	_ = json.Unmarshal(fmt.Appendf(nil, `{"id":%d}`, id), entity)
}
