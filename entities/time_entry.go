package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// TimeEntry represents an Autotask TimeEntry entity.
type TimeEntry struct {
	ID                   autotask.Optional[int64]     `json:"id,omitzero"`
	TicketID             autotask.Optional[int64]     `json:"ticketID,omitzero"`
	ResourceID           autotask.Optional[int64]     `json:"resourceID,omitzero"`
	DateWorked           autotask.Optional[time.Time] `json:"dateWorked,omitzero"`
	StartDateTime        autotask.Optional[time.Time] `json:"startDateTime,omitzero"`
	EndDateTime          autotask.Optional[time.Time] `json:"endDateTime,omitzero"`
	HoursWorked          autotask.Optional[float64]   `json:"hoursWorked,omitzero"`
	SummaryNotes         autotask.Optional[string]    `json:"summaryNotes,omitzero"`
	InternalNotes        autotask.Optional[string]    `json:"internalNotes,omitzero"`
	BillingCodeID        autotask.Optional[int64]     `json:"billingCodeID,omitzero"`
	CreateDateTime       autotask.Optional[time.Time] `json:"createDateTime,omitzero"`
	LastModifiedDateTime autotask.Optional[time.Time] `json:"lastModifiedDateTime,omitzero"`
	UserDefinedFields    []autotask.UDF               `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for time entries.
func (TimeEntry) EntityName() string { return "TimeEntries" }
