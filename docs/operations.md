# CRUD, queries, and pagination

Back to [README](../README.md).

## CRUD operations

All CRUD functions are generic over any type implementing the `Entity` interface:

```go
// Get by ID
ticket, err := autotask.Get[entities.Ticket](ctx, client, 42)

// List with query
tickets, err := autotask.List[entities.Ticket](ctx, client,
	autotask.NewQuery().Where("status", autotask.OpEq, 1),
)

// Count
n, err := autotask.Count[entities.Ticket](ctx, client, autotask.NewQuery())

// Create: the returned entity carries the server-assigned id in ID.
created, err := autotask.Create(ctx, client, &entities.Ticket{
	Title:     autotask.Set("Server down"),
	CompanyID: autotask.Set(int64(123)),
	Status:    autotask.Set(int64(1)),
	Priority:  autotask.Set(int64(2)),
})
if err != nil {
	return err
}
newID, ok := created.ID.Get() // ok is false if the id was not set

// Update
updated, err := autotask.Update(ctx, client, ticket)

// Delete
err = autotask.Delete[entities.Ticket](ctx, client, 42)
```

For retry caveats on `Create` and `CreateChild`, see [Deciding whether to retry](errors.md#deciding-whether-to-retry).

## Optional fields

Autotask fields can be unset, explicitly null, or have a value. `Optional[T]` handles all three states:

```go
ticket := &entities.Ticket{
	Title:    autotask.Set("My ticket"), // set to a value
	Priority: autotask.Null[int64](),    // explicitly null
	// Status is omitted (unset, not sent in the request)
}

if title, ok := ticket.Title.Get(); ok {
	fmt.Println(title)
}
```

## Query builder

```go
q := autotask.NewQuery().
	Where("status", autotask.OpEq, 1).
	Or(
		autotask.Field("priority", autotask.OpEq, 1),
		autotask.Field("priority", autotask.OpEq, 2),
	).
	Fields("id", "title", "status", "priority").
	Limit(50)

tickets, err := autotask.List[entities.Ticket](ctx, client, q)
```

Available operators: `OpEq`, `OpNotEq`, `OpGt`, `OpGte`, `OpLt`, `OpLte`, `OpBeginsWith`, `OpEndsWith`, `OpContains`, `OpExist`, `OpNotExist`, `OpIn`, `OpNotIn`.

Use `WhereUDF` to filter on a user-defined field instead of a standard one:

```go
q := autotask.NewQuery().WhereUDF("MyCustomField", autotask.OpEq, "value")
```

## Iterator pagination

For large result sets, the `*Iter` functions return Go iterators (`iter.Seq2`) that fetch pages on demand and stop fetching as soon as you break out of the range, so you never buffer more than one page at a time. Every list-all function has a matching iterator:

| List-all | Iterator | Entities |
|---|---|---|
| `List` | `ListIter` | typed, top-level |
| `ListChild` | `ListChildIter` | typed, child of a parent |
| `ListRaw` | `ListRawIter` | untyped (`map[string]any`), top-level |
| `ListChildRaw` | `ListChildRawIter` | untyped (`map[string]any`), child of a parent |

`ListChildRawIterOf[P, C]` yields untyped maps like `ListChildRawIter` but builds the URL from the entity types, the same way `ListChildIter` does, so the caller passes no segment strings: `ListChildRawIterOf[entities.Ticket, entities.TicketNote](ctx, client, ticketID)` reads `/v1.0/Tickets/{id}/Notes`.

```go
for ticket, err := range autotask.ListIter[entities.Ticket](ctx, client, autotask.NewQuery()) {
	if err != nil {
		return err
	}
	title, _ := ticket.Title.Get()
	fmt.Println(title)
}
```

`ListRawIter` and `ListChildRawIter` take entity names as strings and yield `map[string]any`, which is handy for entities without a generated Go type:

```go
for item, err := range autotask.ListRawIter(ctx, client, "Tickets", autotask.NewQuery()) {
	if err != nil {
		return err
	}
	fmt.Println(item["id"])
}
```

All paginating loops (including `ListChildRawIterOf`) are bounded by a 1000-page safety limit (`maxPages`); if it is ever hit (for example an API response that cycles `nextPageUrl`), the iterator yields a `*MaxPagesExceededError` rather than looping forever.

The top-level iterators (`ListIter`, `ListRawIter`) do not apply the query's `MaxRecords` cap client-side, unlike `List` and `ListRaw`; break out of the range once you have enough items.

## Raw operations

For entities not defined in the library, use the untyped API:

```go
result, err := autotask.GetRaw(ctx, client, "Companies", 123)
fmt.Println(result["companyName"])

// UpdateRaw returns the response body, for example {"itemId": 123}
resp, err := autotask.UpdateRaw(ctx, client, "Companies",
	map[string]any{"id": 123, "companyName": "Renamed"},
)

results, err := autotask.ListRaw(ctx, client, "Companies",
	autotask.NewQuery().Where("isActive", autotask.OpEq, true),
)
```

## Child entities

```go
// List all notes for a ticket (paginated)
notes, err := autotask.ListChild[entities.Ticket, entities.TicketNote](ctx, client, ticketID)

// Create a note on a ticket
note, err := autotask.CreateChild[entities.Ticket](ctx, client, ticketID, &entities.TicketNote{
	Title:       autotask.Set("Update"),
	Description: autotask.Set("Fixed the issue."),
})

// Update a note through its ticket: PATCH /v1.0/Tickets/{ticketID}/Notes
// The note's ID must be set; every set field is sent.
note.Title = autotask.Set("Update (edited)")
note, err = autotask.UpdateChild[entities.Ticket](ctx, client, ticketID, note)

// Delete an attachment through its note: DELETE /v1.0/TicketNotes/{noteID}/Attachments/{id}
err = autotask.DeleteChild[entities.TicketNote, entities.TicketNoteAttachment](ctx, client, noteID, attachmentID)
```

Some entities are updated or deleted only through the parent's URL; `Update` sends `/v1.0/{Entity}` and `Delete` sends `/v1.0/{Entity}/{id}`, URLs the API does not offer for them. `UpdateChildRaw` and `DeleteChildRaw` do the same with entity names instead of types.

Quote items and expense items are written through their parent as well, under shorter URL segments than their root names. Per the REST swagger (not checked with a live write), `CreateChild[entities.Quote]`, `UpdateChild[entities.Quote]`, and `DeleteChild[entities.Quote, entities.QuoteItem]` address `/v1.0/Quotes/{quoteID}/Items` (the delete appends `/{id}`), and `CreateChild[entities.ExpenseReport]` and `UpdateChild[entities.ExpenseReport]` send `/v1.0/Expenses/{reportID}/Items`; the API offers no expense item delete. A child type whose URL names its parent with a segment other than the parent's `EntityName` implements `ChildParentEntity` (`ChildParentName()`, as `ExpenseItem` does), and the typed child calls use that segment for the parent.
