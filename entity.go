package autotask

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

// EntityWithID is an optional interface that entities can implement
// to receive the server-generated ID from Create operations.
// The Autotask API returns {"itemId": N} on successful creates.
type EntityWithID interface {
	SetID(id int64)
}
