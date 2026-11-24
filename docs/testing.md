# Testing

Back to [README](../README.md).

## Mock client with fixtures

Use `autotasktest.NewMockClient` to create an in-memory client for tests:

```go
import "github.com/tphakala/go-autotask/autotasktest"

func TestMyCode(t *testing.T) {
	client := autotasktest.NewMockClient(t,
		autotasktest.WithFixture("GET", "/v1.0/Tickets/42", 200, map[string]any{
			"item": map[string]any{"id": 42, "title": "Test"},
		}),
	)
	// use client in tests; server and client are cleaned up automatically
}
```

## Validating in-memory server

`autotasktest.NewServer` is a validating in-memory server that keeps what tests create. It stores child items per parent and per entity type: a child URL such as `/v1.0/Tickets/{id}/Attachments` reaches the store of the `TicketAttachments` entity (the singular parent name plus the segment), so the same items answer `Get`, `List`, and `Count` for that type. A type whose entity name is the segment itself (`Contact` under `Companies`, `Task` under `Projects`) reaches its own store as well.

Every entity type in the `entities` package resolves this way, unless a store named `singular(Parent)+Segment` for another type already exists. The mock sees only the URL, so a type from outside this module on a URL that names an `entities` type this way goes to that type's store and needs `WithChildEntity`. On any other URL, a type from outside this module that fits neither name needs `WithChildEntity`, or a `WithEntity` seed when its entity name is the segment.

A child list shows the items created under that parent collection and id, child `GET` by id, `PATCH`, and `DELETE` check both, and a list pages by `?page=N`. An item created at `/v1.0/Companies/9/Contacts` is not listed at `/v1.0/Opportunities/9/Contacts`, although both URLs reach the `Contacts` store. `WithChildEntity[Parent](parentID, items...)` seeds children under a parent, so they belong to the parent collection of its route (a `parentID` of 0 means no parent), and is also how to route a child type whose entity name is neither of those names; it uses the child's `ChildParentName` for the parent half of the route when the child type has one. Items seeded with `WithEntity` have no parent and appear under every parent whose child URL resolves to their store. `DELETE` needs `WithDeleteSupport` with the child's entity name, for example `WithDeleteSupport("TicketNoteAttachments")`.

## Simulating API write restrictions

By default the mock accepts a create or update at any URL that resolves. Three options make a test fail the way the API would:

- **`WithChildOnlyWrites[Parent, Child]()`**: for example `WithChildOnlyWrites[entities.Company, entities.Contact]()`, accepts writes of that type only on the URL the typed child calls send. A root write, a child write that resolves to that type's store under another parent or segment, and a create under a segment the type uses that matches no store and no `entities` type answer 400, while a PATCH or DELETE on a URL that resolves to no store answers 404 as before. Under a segment that a `WithChildOnlyWrites` type uses (`Contacts`, `Items`), a create that matches no store and no `entities` type is rejected instead of making a store, so a type from outside this module there needs `WithChildEntity`.
- **`WithoutCreateSupport(entityName)`** and **`WithoutUpdateSupport(entityName)`**: reject POST or PATCH for the named entity at the root URL and at every child URL that resolves to its store.

`WithDeleteSupport`, `WithoutCreateSupport`, and `WithoutUpdateSupport` also set the flags of the default `entityInformation` answer, and `WithEntityMetadata` replaces that answer. A metadata path under a child URL (`/v1.0/Tickets/{id}/AdditionalConfigurationItems/entityInformation`, and its `/fields` and `/userDefinedFields`) answers for the child entity that URL resolves to, so `WithEntityMetadata` takes the child's entity name (`TicketAdditionalConfigurationItems`). The rejections answer 400, like the unsupported-delete answer. The exact status and message of the live API have not been verified, so assert that the write fails, not its type.
