package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ConfigurationItemCategory represents an Autotask ConfigurationItemCategory
// entity, a category of configuration items.
//
// It is a root entity: autotask.Get, autotask.List, autotask.Count,
// autotask.Create and autotask.Update reach the operations the REST swagger
// lists for it on /v1.0/ConfigurationItemCategories. The swagger lists no
// delete (canDelete is false in the entity metadata), so deactivate a
// category with IsActive instead of passing it to autotask.Delete. It has no
// user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not
// regenerate it.
type ConfigurationItemCategory struct {
	DisplayColorRgb       autotask.Optional[int64]  `json:"displayColorRgb,omitzero"`
	ID                    autotask.Optional[int64]  `json:"id,omitzero"`
	IsActive              autotask.Optional[bool]   `json:"isActive,omitzero"`
	IsClientPortalDefault autotask.Optional[bool]   `json:"isClientPortalDefault,omitzero"`
	IsGlobalDefault       autotask.Optional[bool]   `json:"isGlobalDefault,omitzero"`
	Name                  autotask.Optional[string] `json:"name,omitzero"`
	Nickname              autotask.Optional[string] `json:"nickname,omitzero"`
}

// EntityName returns the Autotask API entity name for configuration item categories.
func (ConfigurationItemCategory) EntityName() string { return "ConfigurationItemCategories" }
