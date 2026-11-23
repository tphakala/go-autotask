package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ResourceRoleQueue represents an Autotask ResourceRoleQueue entity, the
// assignment of a resource to a queue.
//
// Get, List and Count work at the root. Per the REST swagger the writes go
// through the resource: autotask.ListChild[Resource, ResourceRoleQueue] lists
// /v1.0/Resources/{resourceID}/RoleQueues, autotask.CreateChild[Resource] posts
// to that path, and autotask.UpdateChild[Resource] sends PATCH to it with the
// assignment's ID in the body. The API has no delete (canDelete is false in the
// entity information and the swagger lists no DELETE). It has no user-defined
// fields.
//
// ResourceID is required and read-only per the field metadata. Whether the API
// needs it in the body when the URL names the resource has not been checked
// against a live tenant.
//
// The field QueueID takes a value of the field's picklist; list the values with
// metadata.GetPickList.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type ResourceRoleQueue struct {
	ID         autotask.Optional[int64] `json:"id,omitzero"`
	QueueID    autotask.Optional[int64] `json:"queueID,omitzero"`
	ResourceID autotask.Optional[int64] `json:"resourceID,omitzero"`
}

// EntityName returns the Autotask API entity name for resource role queues.
func (ResourceRoleQueue) EntityName() string { return "ResourceRoleQueues" }
