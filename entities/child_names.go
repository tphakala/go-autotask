package entities

// ChildEntityName methods for entities whose child URL segment differs from
// their direct CRUD EntityName. The Autotask REST API uses shorter names in
// child resource URLs, e.g. /v1.0/Tickets/{parentId}/Notes (not /TicketNotes).

// childNameNotes is the shared child URL segment for note entities.
const childNameNotes = "Notes"

func (TicketNote) ChildEntityName() string       { return childNameNotes }
func (ProjectNote) ChildEntityName() string      { return childNameNotes }
func (CompanyNote) ChildEntityName() string      { return childNameNotes }
func (TicketAttachment) ChildEntityName() string { return "Attachments" }
