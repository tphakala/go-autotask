package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ContactGroup represents an Autotask ContactGroup entity, a named group of
// contacts.
//
// It is a root entity: Get, List and Count work, autotask.Create sends POST to
// /v1.0/ContactGroups, autotask.Update sends PATCH to it, and autotask.Delete
// sends DELETE to /v1.0/ContactGroups/{id}, all listed in the REST swagger. Its
// members are ContactGroupContact items, reached through the group. It has no
// user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type ContactGroup struct {
	ID       autotask.Optional[int64]  `json:"id,omitzero"`
	IsActive autotask.Optional[bool]   `json:"isActive,omitzero"`
	Name     autotask.Optional[string] `json:"name,omitzero"`
}

// EntityName returns the Autotask API entity name for contact groups.
func (ContactGroup) EntityName() string { return "ContactGroups" }
