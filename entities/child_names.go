package entities

// ChildEntityName methods for entities whose child URL segment differs from
// their direct CRUD EntityName. The Autotask REST API uses shorter names in
// child resource URLs, e.g. /v1.0/Tickets/{parentId}/Notes (not /TicketNotes).

// childNameNotes is the shared child URL segment for note entities.
const childNameNotes = "Notes"

// childNameAttachments is the shared child URL segment for attachment entities.
const childNameAttachments = "Attachments"

// childNameItems is the shared child URL segment for quote and expense items.
const childNameItems = "Items"

func (TicketNote) ChildEntityName() string           { return childNameNotes }
func (ProjectNote) ChildEntityName() string          { return childNameNotes }
func (CompanyNote) ChildEntityName() string          { return childNameNotes }
func (TicketAttachment) ChildEntityName() string     { return childNameAttachments }
func (TicketNoteAttachment) ChildEntityName() string { return childNameAttachments }

// ChildEntityName returns the URL segment of the configuration item category's
// UDF association collection.
func (ConfigurationItemCategoryUdfAssociation) ChildEntityName() string { return "UdfAssociations" }

// ChildEntityName returns the shared note segment.
func (ConfigurationItemNote) ChildEntityName() string { return childNameNotes }

// ChildEntityName returns the shared attachment segment.
func (ConfigurationItemAttachment) ChildEntityName() string { return childNameAttachments }

// ChildEntityName returns the shared attachment segment.
func (ConfigurationItemNoteAttachment) ChildEntityName() string { return childNameAttachments }

// ChildEntityName returns the URL segment of a configuration item's related
// item collection.
func (ConfigurationItemRelatedItem) ChildEntityName() string { return "RelatedItems" }

// ChildEntityName returns the URL segment of a configuration item's billing
// product association collection.
func (ConfigurationItemBillingProductAssociation) ChildEntityName() string {
	return "BillingProductAssociations"
}

// ChildEntityName returns the URL segment of a configuration item's DNS record
// collection.
func (ConfigurationItemDnsRecord) ChildEntityName() string { return "DnsRecords" }

// ChildEntityName returns the URL segment of a configuration item's SSL
// subject alternative name collection.
func (ConfigurationItemSslSubjectAlternativeName) ChildEntityName() string {
	return "SslSubjectAlternativeNames"
}

// ChildEntityName returns the URL segment of a ticket's additional
// configuration item collection.
func (TicketAdditionalConfigurationItem) ChildEntityName() string {
	return "AdditionalConfigurationItems"
}

// ChildEntityName returns the URL segment of a resource's role department
// collection.
func (ResourceRoleDepartment) ChildEntityName() string { return "RoleDepartments" }

// ChildEntityName returns the URL segment of a resource's role queue
// collection.
func (ResourceRoleQueue) ChildEntityName() string { return "RoleQueues" }

// ChildEntityName returns the URL segment of a resource's service desk role
// collection.
func (ResourceServiceDeskRole) ChildEntityName() string { return "ServiceDeskRoles" }

// ChildEntityName returns the URL segment of a resource's skill collection.
func (ResourceSkill) ChildEntityName() string { return "Skills" }

// ChildEntityName returns the URL segment of a resource's role collection.
func (ResourceRole) ChildEntityName() string { return "Roles" }

// ChildEntityName returns the shared attachment segment.
func (ResourceAttachment) ChildEntityName() string { return childNameAttachments }

// ChildEntityName returns the URL segment of a contact group's member
// collection.
func (ContactGroupContact) ChildEntityName() string { return "Contacts" }

// ChildEntityName returns the URL segment of a quote's item collection.
//
// Per the REST swagger (not checked with a live write), quote items are
// written through their quote: autotask.CreateChild[Quote] posts to
// /v1.0/Quotes/{quoteID}/Items, autotask.UpdateChild[Quote] sends PATCH to it
// with the item's ID in the body, and
// autotask.DeleteChild[Quote, QuoteItem] sends DELETE to
// /v1.0/Quotes/{quoteID}/Items/{id}. autotask.Create, autotask.Update and
// autotask.Delete send root URLs the swagger does not list for this entity.
// autotask.Get, autotask.List and autotask.Count work at the root.
func (QuoteItem) ChildEntityName() string { return childNameItems }

// ChildEntityName returns the URL segment of an expense report's item
// collection.
//
// Per the REST swagger (not checked with a live write), expense items are
// written through their expense report: autotask.CreateChild[ExpenseReport]
// posts to /v1.0/Expenses/{reportID}/Items and autotask.UpdateChild[ExpenseReport]
// sends PATCH to it with the item's ID in the body. The swagger lists no
// delete for expense items. autotask.Create, autotask.Update and
// autotask.Delete send root URLs the swagger does not list for this entity.
// autotask.Get, autotask.List and autotask.Count work at the root.
func (ExpenseItem) ChildEntityName() string { return childNameItems }

// ChildParentName returns the segment that names the expense report in child
// URLs: /v1.0/Expenses/{reportID}/Items, not the root EntityName
// "ExpenseReports". See autotask.ChildParentEntity.
func (ExpenseItem) ChildParentName() string { return "Expenses" }
