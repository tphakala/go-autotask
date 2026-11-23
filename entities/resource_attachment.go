package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// ResourceAttachment represents an Autotask ResourceAttachment entity, a file
// attached to a resource.
//
// Get, List and Count work at the root. Per the REST swagger, it is reached
// through its parent resource: autotask.ListChild[Resource, ResourceAttachment]
// lists /v1.0/Resources/{resourceID}/Attachments,
// autotask.CreateChild[Resource] posts to that path, and
// autotask.DeleteChild[Resource, ResourceAttachment] sends DELETE to that path
// plus /{id}. The API has no update for this entity. autotask.Create,
// autotask.Update and autotask.Delete send root URLs that the REST swagger does
// not list for it, so use the child calls. The file content goes base64-encoded
// in Data: the REST swagger types data as a string with format byte, and this
// package carries it as a string like ConfigurationItemAttachment.Data. It has
// no user-defined fields.
//
// The fields AttachmentType, CreatorType, Publish take a value of the field's
// picklist; list the values with metadata.GetPickList.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type ResourceAttachment struct {
	AttachDate                    autotask.Optional[time.Time] `json:"attachDate,omitzero"`
	AttachedByContactID           autotask.Optional[int64]     `json:"attachedByContactID,omitzero"`
	AttachedByResourceID          autotask.Optional[int64]     `json:"attachedByResourceID,omitzero"`
	AttachmentType                autotask.Optional[string]    `json:"attachmentType,omitzero"`
	ContentType                   autotask.Optional[string]    `json:"contentType,omitzero"`
	CreatorType                   autotask.Optional[int64]     `json:"creatorType,omitzero"`
	Data                          autotask.Optional[string]    `json:"data,omitzero"`
	FileSize                      autotask.Optional[int64]     `json:"fileSize,omitzero"`
	FullPath                      autotask.Optional[string]    `json:"fullPath,omitzero"`
	ID                            autotask.Optional[int64]     `json:"id,omitzero"`
	ImpersonatorCreatorResourceID autotask.Optional[int64]     `json:"impersonatorCreatorResourceID,omitzero"`
	OpportunityID                 autotask.Optional[int64]     `json:"opportunityID,omitzero"`
	ParentID                      autotask.Optional[int64]     `json:"parentID,omitzero"`
	Publish                       autotask.Optional[int64]     `json:"publish,omitzero"`
	ResourceID                    autotask.Optional[int64]     `json:"resourceID,omitzero"`
	Title                         autotask.Optional[string]    `json:"title,omitzero"`
}

// EntityName returns the Autotask API entity name for resource attachments.
func (ResourceAttachment) EntityName() string { return "ResourceAttachments" }
