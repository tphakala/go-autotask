package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// Task represents an Autotask project Task entity.
// Note: This is an Autotask project task, not to be confused with Go tasks.
type Task struct {
	ID                   autotask.Optional[int64]     `json:"id,omitzero"`
	Title                autotask.Optional[string]    `json:"title,omitzero"`
	Description          autotask.Optional[string]    `json:"description,omitzero"`
	Status               autotask.Optional[int64]       `json:"status,omitzero"`
	Priority             autotask.Optional[int64]       `json:"priority,omitzero"`
	ProjectID            autotask.Optional[int64]     `json:"projectID,omitzero"`
	AssignedResourceID   autotask.Optional[int64]     `json:"assignedResourceID,omitzero"`
	EstimatedHours       autotask.Optional[float64]   `json:"estimatedHours,omitzero"`
	StartDateTime        autotask.Optional[time.Time] `json:"startDateTime,omitzero"`
	EndDateTime          autotask.Optional[time.Time] `json:"endDateTime,omitzero"`
	CreateDateTime       autotask.Optional[time.Time] `json:"createDateTime,omitzero"`
	LastActivityDateTime autotask.Optional[time.Time] `json:"lastActivityDateTime,omitzero"`
	CompletedDateTime    autotask.Optional[time.Time] `json:"completedDateTime,omitzero"`
	UserDefinedFields    []autotask.UDF               `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for tasks.
func (Task) EntityName() string { return "Tasks" }
