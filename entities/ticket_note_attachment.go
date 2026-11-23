//nolint:dupl // the two note attachment entities share most of their field list
package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// TicketNoteAttachment represents an Autotask TicketNoteAttachment entity, a
// file attached to a ticket note. Per
// https://www.autotask.net/help/developerhelp/Content/APIs/REST/Entities/TicketNoteAttachmentsEntity.htm,
// it is created through its parent note: autotask.CreateChild[TicketNote]
// posts it to /v1.0/TicketNotes/{noteID}/Attachments, and
// autotask.ListChild[TicketNote, TicketNoteAttachment] lists a note's
// attachments from the same path. The page sends deletes through the parent
// too: autotask.DeleteChild[TicketNote, TicketNoteAttachment] sends
// DELETE /v1.0/TicketNotes/{noteID}/Attachments/{id}, while autotask.Delete
// sends DELETE /v1.0/TicketNoteAttachments/{id}, which the page does not list.
// The file content goes base64-encoded in Data. The API has no update for this entity,
// and it has no user-defined fields.
//
// This file is hand-written. Its fields follow the documentation page above;
// it was not generated from the entity's field metadata.
type TicketNoteAttachment struct {
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
	TicketID                      autotask.Optional[int64]     `json:"ticketID,omitzero"`
	TicketNoteID                  autotask.Optional[int64]     `json:"ticketNoteID,omitzero"`
	Title                         autotask.Optional[string]    `json:"title,omitzero"`
}

func (TicketNoteAttachment) EntityName() string { return "TicketNoteAttachments" }
