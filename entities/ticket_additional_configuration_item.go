package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// TicketAdditionalConfigurationItem represents an Autotask
// TicketAdditionalConfigurationItem entity, the link between a ticket and an
// additional configuration item.
//
// Per the REST swagger, it is reached through its parent ticket:
// autotask.ListChild[Ticket, TicketAdditionalConfigurationItem] lists
// /v1.0/Tickets/{ticketID}/AdditionalConfigurationItems,
// autotask.CreateChild[Ticket] posts to that path, and
// autotask.DeleteChild[Ticket, TicketAdditionalConfigurationItem] sends
// DELETE to that path plus /{id}. The API has no update for this entity.
// autotask.Create, autotask.Update and autotask.Delete send root URLs that
// the REST swagger does not list for this entity, so use the child calls. It
// has no user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not
// regenerate it.
type TicketAdditionalConfigurationItem struct {
	ConfigurationItemID autotask.Optional[int64] `json:"configurationItemID,omitzero"`
	ID                  autotask.Optional[int64] `json:"id,omitzero"`
	TicketID            autotask.Optional[int64] `json:"ticketID,omitzero"`
}

// EntityName returns the Autotask API entity name for ticket additional configuration items.
func (TicketAdditionalConfigurationItem) EntityName() string {
	return "TicketAdditionalConfigurationItems"
}
