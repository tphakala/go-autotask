package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ResourceServiceDeskRole represents an Autotask ResourceServiceDeskRole
// entity, a service desk role of a resource.
//
// Get, List and Count work at the root. Per the REST swagger the writes go
// through the resource: autotask.ListChild[Resource, ResourceServiceDeskRole]
// lists /v1.0/Resources/{resourceID}/ServiceDeskRoles,
// autotask.CreateChild[Resource] posts to that path, and
// autotask.UpdateChild[Resource] sends PATCH to it with the role's ID in the
// body. The API has no delete (canDelete is false in the entity information and
// the swagger lists no DELETE); set IsActive to false to retire a role (not
// checked against a live tenant). It has no user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type ResourceServiceDeskRole struct {
	ID         autotask.Optional[int64] `json:"id,omitzero"`
	IsActive   autotask.Optional[bool]  `json:"isActive,omitzero"`
	IsDefault  autotask.Optional[bool]  `json:"isDefault,omitzero"`
	ResourceID autotask.Optional[int64] `json:"resourceID,omitzero"`
	RoleID     autotask.Optional[int64] `json:"roleID,omitzero"`
}

// EntityName returns the Autotask API entity name for resource service desk roles.
func (ResourceServiceDeskRole) EntityName() string { return "ResourceServiceDeskRoles" }
