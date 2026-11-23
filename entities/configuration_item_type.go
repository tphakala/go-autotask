package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ConfigurationItemType represents an Autotask ConfigurationItemType entity,
// a type that classifies configuration items.
//
// It is a root entity: autotask.Get, autotask.List, autotask.Count,
// autotask.Create, autotask.Update and autotask.Delete reach the operations
// the REST swagger lists for it on /v1.0/ConfigurationItemTypes. It has no
// user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not
// regenerate it.
type ConfigurationItemType struct {
	ID       autotask.Optional[int64]  `json:"id,omitzero"`
	IsActive autotask.Optional[bool]   `json:"isActive,omitzero"`
	Name     autotask.Optional[string] `json:"name,omitzero"`
}

// EntityName returns the Autotask API entity name for configuration item types.
func (ConfigurationItemType) EntityName() string { return "ConfigurationItemTypes" }
