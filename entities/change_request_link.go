package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ChangeRequestLink represents an Autotask ChangeRequestLink entity, which
// associates a Change Request ticket with a Problem or Incident ticket.
//
// Per the ChangeRequestLinks entity reference
// (https://www.autotask.net/help/developerhelp/Content/APIs/REST/Entities/ChangeRequestLinksEntity.htm),
// ChangeRequestTicketID must name a ticket whose ticketType is Change Request
// and ProblemOrIncidentTicketID one whose ticketType is Problem or Incident.
// A Change Request cannot be linked to a Service Request or to another Change
// Request.
//
// The API supports create, query, count, get and delete for this entity but
// not update: the reference leaves "Can Update" blank and the REST swagger
// lists no PATCH or PUT for /V1.0/ChangeRequestLinks. Do not pass it to
// autotask.Update; delete the link and create a new one instead. What
// Autotask answers to such a PATCH is NOT MEASURED. The entity has no
// user-defined fields, so the struct has no UserDefinedFields.
type ChangeRequestLink struct {
	ID                        autotask.Optional[int64] `json:"id,omitzero"`
	ChangeRequestTicketID     autotask.Optional[int64] `json:"changeRequestTicketID,omitzero"`
	ProblemOrIncidentTicketID autotask.Optional[int64] `json:"problemOrIncidentTicketID,omitzero"`
}

// EntityName returns the Autotask API entity name for change request links.
func (ChangeRequestLink) EntityName() string { return "ChangeRequestLinks" }
