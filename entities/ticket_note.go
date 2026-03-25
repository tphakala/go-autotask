package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// TicketNote represents an Autotask TicketNote entity.
type TicketNote struct {
	ID                autotask.Optional[int64]     `json:"id,omitzero"`
	TicketID          autotask.Optional[int64]     `json:"ticketID,omitzero"`
	Title             autotask.Optional[string]    `json:"title,omitzero"`
	Description       autotask.Optional[string]    `json:"description,omitzero"`
	NoteType          autotask.Optional[int64]       `json:"noteType,omitzero"`
	Publish           autotask.Optional[int64]       `json:"publish,omitzero"`
	CreateDateTime    autotask.Optional[time.Time] `json:"createDateTime,omitzero"`
	CreatorResourceID autotask.Optional[int64]     `json:"creatorResourceID,omitzero"`
	LastActivityDate  autotask.Optional[time.Time] `json:"lastActivityDate,omitzero"`
	UserDefinedFields []autotask.UDF              `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for ticket notes.
func (TicketNote) EntityName() string { return "TicketNotes" }
