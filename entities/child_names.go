package entities

// ChildEntityName methods for entities whose child URL segment differs from
// their direct CRUD EntityName. The Autotask REST API uses shorter names in
// child resource URLs, e.g. /v1.0/Tickets/{parentId}/Notes (not /TicketNotes).

func (TicketNote) ChildEntityName() string      { return "Notes" }
func (ProjectNote) ChildEntityName() string      { return "Notes" }
func (CompanyNote) ChildEntityName() string      { return "Notes" }
func (TicketAttachment) ChildEntityName() string { return "Attachments" }
