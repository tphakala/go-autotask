package entities

// ChildEntityName methods for entities whose child URL segment differs from
// their direct CRUD EntityName. The Autotask REST API uses shorter names in
// child resource URLs, e.g. /v1.0/Tickets/{parentId}/Notes (not /TicketNotes).

// childNameNotes is the shared child URL segment for note entities.
const childNameNotes = "Notes"

// childNameAttachments is the shared child URL segment for attachment entities.
const childNameAttachments = "Attachments"

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
