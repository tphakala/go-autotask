package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// Resource represents an Autotask Resource entity.
type Resource struct {
	ID                        autotask.Optional[int64]  `json:"id,omitzero"`
	FirstName                 autotask.Optional[string] `json:"firstName,omitzero"`
	LastName                  autotask.Optional[string] `json:"lastName,omitzero"`
	Email                     autotask.Optional[string] `json:"email,omitzero"`
	UserName                  autotask.Optional[string] `json:"userName,omitzero"`
	Title                     autotask.Optional[string] `json:"title,omitzero"`
	IsActive                  autotask.Optional[bool]   `json:"isActive,omitzero"`
	ResourceType              autotask.Optional[int64]  `json:"resourceType,omitzero"`
	LocationID                autotask.Optional[int64]  `json:"locationID,omitzero"`
	DefaultServiceDeskRoleID  autotask.Optional[int64]  `json:"defaultServiceDeskRoleID,omitzero"`
	UserDefinedFields         []autotask.UDF            `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for resources.
func (Resource) EntityName() string { return "Resources" }
