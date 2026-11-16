package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ChangeRequestLink represents an Autotask ChangeRequestLink entity, which
// links a Change Request ticket to a Problem or Incident ticket. Per
// https://www.autotask.net/help/developerhelp/Content/APIs/REST/Entities/ChangeRequestLinksEntity.htm,
// ChangeRequestTicketID must name a Change Request ticket and
// ProblemOrIncidentTicketID a Problem or Incident ticket.
//
// The API has no update for this entity (no PATCH or PUT in the REST
// swagger), so do not pass it to autotask.Update; delete the link and create
// a new one. It has no user-defined fields.
//
// This file is hand-written: autotask-gen always emits UserDefinedFields, so
// do not regenerate it.
type ChangeRequestLink struct {
	ID                        autotask.Optional[int64] `json:"id,omitzero"`
	ChangeRequestTicketID     autotask.Optional[int64] `json:"changeRequestTicketID,omitzero"`
	ProblemOrIncidentTicketID autotask.Optional[int64] `json:"problemOrIncidentTicketID,omitzero"`
}

// EntityName returns the Autotask API entity name for change request links.
func (ChangeRequestLink) EntityName() string { return "ChangeRequestLinks" }
