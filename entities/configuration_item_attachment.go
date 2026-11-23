package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// ConfigurationItemAttachment represents an Autotask
// ConfigurationItemAttachment entity, a file attached to a configuration
// item.
//
// Per the REST swagger, it is reached through its parent configuration item:
// autotask.ListChild[ConfigurationItem, ConfigurationItemAttachment] lists
// /v1.0/ConfigurationItems/{ciID}/Attachments,
// autotask.CreateChild[ConfigurationItem] posts to that path, and
// autotask.DeleteChild[ConfigurationItem, ConfigurationItemAttachment] sends
// DELETE to that path plus /{id}. The API has no update for this entity.
// autotask.Create, autotask.Update and autotask.Delete send root URLs that
// the REST swagger does not list for this entity, so use the child calls.
// The file content goes base64-encoded in Data: the REST swagger types data
// as a string with format byte, and this package carries it as a string like
// TicketNoteAttachment.Data. It has no user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not
// regenerate it.
type ConfigurationItemAttachment struct {
	AttachDate                    autotask.Optional[time.Time] `json:"attachDate,omitzero"`
	AttachedByContactID           autotask.Optional[int64]     `json:"attachedByContactID,omitzero"`
	AttachedByResourceID          autotask.Optional[int64]     `json:"attachedByResourceID,omitzero"`
	AttachmentType                autotask.Optional[string]    `json:"attachmentType,omitzero"`
	ConfigurationItemID           autotask.Optional[int64]     `json:"configurationItemID,omitzero"`
	ConfigurationItemNoteID       autotask.Optional[int64]     `json:"configurationItemNoteID,omitzero"`
	ContentType                   autotask.Optional[string]    `json:"contentType,omitzero"`
	CreatorType                   autotask.Optional[int64]     `json:"creatorType,omitzero"`
	Data                          autotask.Optional[string]    `json:"data,omitzero"`
	FileSize                      autotask.Optional[int64]     `json:"fileSize,omitzero"`
	FullPath                      autotask.Optional[string]    `json:"fullPath,omitzero"`
	ID                            autotask.Optional[int64]     `json:"id,omitzero"`
	ImpersonatorCreatorResourceID autotask.Optional[int64]     `json:"impersonatorCreatorResourceID,omitzero"`
	OpportunityID                 autotask.Optional[int64]     `json:"opportunityID,omitzero"`
	ParentAttachmentID            autotask.Optional[int64]     `json:"parentAttachmentID,omitzero"`
	ParentID                      autotask.Optional[int64]     `json:"parentID,omitzero"`
	Publish                       autotask.Optional[int64]     `json:"publish,omitzero"`
	Title                         autotask.Optional[string]    `json:"title,omitzero"`
}

// EntityName returns the Autotask API entity name for configuration item attachments.
func (ConfigurationItemAttachment) EntityName() string { return "ConfigurationItemAttachments" }
