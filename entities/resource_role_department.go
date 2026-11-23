package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ResourceRoleDepartment represents an Autotask ResourceRoleDepartment entity,
// the assignment of a resource to a department with a role.
//
// Get, List and Count work at the root. Per the REST swagger the writes go
// through the resource: autotask.ListChild[Resource, ResourceRoleDepartment]
// lists /v1.0/Resources/{resourceID}/RoleDepartments,
// autotask.CreateChild[Resource] posts to that path, and
// autotask.UpdateChild[Resource] sends PATCH to it with the assignment's ID in
// the body. The API has no delete (canDelete is false in the entity information
// and the swagger lists no DELETE); set IsActive to false to retire an
// assignment (not checked against a live tenant). It has no user-defined
// fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type ResourceRoleDepartment struct {
	DepartmentID     autotask.Optional[int64] `json:"departmentID,omitzero"`
	ID               autotask.Optional[int64] `json:"id,omitzero"`
	IsActive         autotask.Optional[bool]  `json:"isActive,omitzero"`
	IsDefault        autotask.Optional[bool]  `json:"isDefault,omitzero"`
	IsDepartmentLead autotask.Optional[bool]  `json:"isDepartmentLead,omitzero"`
	ResourceID       autotask.Optional[int64] `json:"resourceID,omitzero"`
	RoleID           autotask.Optional[int64] `json:"roleID,omitzero"`
}

// EntityName returns the Autotask API entity name for resource role departments.
func (ResourceRoleDepartment) EntityName() string { return "ResourceRoleDepartments" }
