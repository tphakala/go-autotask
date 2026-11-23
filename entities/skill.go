package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// Skill represents an Autotask Skill entity, a skill that can be assigned to
// resources.
//
// It is read-only: Get, List and Count work at the root, and the entity
// information reports canCreate, canUpdate and canDelete as false. It has
// no user-defined fields.
//
// The field CategoryID takes a value of the field's picklist; list the values
// with metadata.GetPickList.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type Skill struct {
	CategoryID  autotask.Optional[int64]  `json:"categoryID,omitzero"`
	Description autotask.Optional[string] `json:"description,omitzero"`
	ID          autotask.Optional[int64]  `json:"id,omitzero"`
	IsActive    autotask.Optional[bool]   `json:"isActive,omitzero"`
	Name        autotask.Optional[string] `json:"name,omitzero"`
}

// EntityName returns the Autotask API entity name for skills.
func (Skill) EntityName() string { return "Skills" }
