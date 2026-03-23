package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// Project represents an Autotask Project entity.
type Project struct {
	ID                   autotask.Optional[int64]     `json:"id,omitzero"`
	ProjectName          autotask.Optional[string]    `json:"projectName,omitzero"`
	Description          autotask.Optional[string]    `json:"description,omitzero"`
	Status               autotask.Optional[int]       `json:"status,omitzero"`
	Type                 autotask.Optional[int]       `json:"type,omitzero"`
	CompanyID            autotask.Optional[int64]     `json:"companyID,omitzero"`
	StartDateTime        autotask.Optional[time.Time] `json:"startDateTime,omitzero"`
	EndDateTime          autotask.Optional[time.Time] `json:"endDateTime,omitzero"`
	EstimatedHours       autotask.Optional[float64]   `json:"estimatedHours,omitzero"`
	ActualHours          autotask.Optional[float64]   `json:"actualHours,omitzero"`
	CreateDateTime       autotask.Optional[time.Time] `json:"createDateTime,omitzero"`
	LastActivityDateTime autotask.Optional[time.Time] `json:"lastActivityDateTime,omitzero"`
	UserDefinedFields    []autotask.UDF               `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for projects.
func (Project) EntityName() string { return "Projects" }
