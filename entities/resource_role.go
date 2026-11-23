package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ResourceRole represents an Autotask ResourceRole entity, a role assigned to a
// resource.
//
// It is read-only: Get, List and Count work at the root, and
// autotask.ListChild[Resource, ResourceRole] lists
// /v1.0/Resources/{resourceID}/Roles. The API has no create, update or delete
// for it (the entity information reports canCreate, canUpdate and canDelete as
// false). It has no user-defined fields.
//
// The field QueueID takes a value of the field's picklist; list the values with
// metadata.GetPickList.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type ResourceRole struct {
	DepartmentID autotask.Optional[int64] `json:"departmentID,omitzero"`
	ID           autotask.Optional[int64] `json:"id,omitzero"`
	IsActive     autotask.Optional[bool]  `json:"isActive,omitzero"`
	QueueID      autotask.Optional[int64] `json:"queueID,omitzero"`
	ResourceID   autotask.Optional[int64] `json:"resourceID,omitzero"`
	RoleID       autotask.Optional[int64] `json:"roleID,omitzero"`
}

// EntityName returns the Autotask API entity name for resource roles.
func (ResourceRole) EntityName() string { return "ResourceRoles" }
