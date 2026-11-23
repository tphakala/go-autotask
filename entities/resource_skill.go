package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ResourceSkill represents an Autotask ResourceSkill entity, a skill of a
// resource and its level.
//
// Get, List and Count work at the root. Per the REST swagger,
// autotask.ListChild[Resource, ResourceSkill] lists
// /v1.0/Resources/{resourceID}/Skills and autotask.UpdateChild[Resource] sends
// PATCH to that path with the skill row's ID in the body. The API has no create
// or delete (canCreate and canDelete are false in the entity information and
// the swagger lists no POST or DELETE on that path). It has no user-defined
// fields.
//
// The field SkillLevel takes a value of the field's picklist; list the values
// with metadata.GetPickList.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type ResourceSkill struct {
	ID               autotask.Optional[int64]  `json:"id,omitzero"`
	ResourceID       autotask.Optional[int64]  `json:"resourceID,omitzero"`
	SkillDescription autotask.Optional[string] `json:"skillDescription,omitzero"`
	SkillID          autotask.Optional[int64]  `json:"skillID,omitzero"`
	SkillLevel       autotask.Optional[int64]  `json:"skillLevel,omitzero"`
}

// EntityName returns the Autotask API entity name for resource skills.
func (ResourceSkill) EntityName() string { return "ResourceSkills" }
