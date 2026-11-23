package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// ConfigurationItemNote represents an Autotask ConfigurationItemNote entity,
// a note on a configuration item.
//
// Per the REST swagger, it is reached through its parent configuration item:
// autotask.ListChild[ConfigurationItem, ConfigurationItemNote] lists
// /v1.0/ConfigurationItems/{ciID}/Notes,
// autotask.CreateChild[ConfigurationItem] posts to that path, and
// autotask.UpdateChild[ConfigurationItem] sends PATCH to it with the note's
// ID in the body. The swagger lists no delete for notes. autotask.Create,
// autotask.Update and autotask.Delete send root URLs that the REST swagger
// does not list for this entity, so use the child calls. It has no
// user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not
// regenerate it.
type ConfigurationItemNote struct {
	ConfigurationItemID           autotask.Optional[int64]     `json:"configurationItemID,omitzero"`
	CreateDateTime                autotask.Optional[time.Time] `json:"createDateTime,omitzero"`
	CreatorResourceID             autotask.Optional[int64]     `json:"creatorResourceID,omitzero"`
	Description                   autotask.Optional[string]    `json:"description,omitzero"`
	ID                            autotask.Optional[int64]     `json:"id,omitzero"`
	ImpersonatorCreatorResourceID autotask.Optional[int64]     `json:"impersonatorCreatorResourceID,omitzero"`
	ImpersonatorUpdaterResourceID autotask.Optional[int64]     `json:"impersonatorUpdaterResourceID,omitzero"`
	LastActivityDate              autotask.Optional[time.Time] `json:"lastActivityDate,omitzero"`
	NoteType                      autotask.Optional[int64]     `json:"noteType,omitzero"`
	Title                         autotask.Optional[string]    `json:"title,omitzero"`
}

// EntityName returns the Autotask API entity name for configuration item notes.
func (ConfigurationItemNote) EntityName() string { return "ConfigurationItemNotes" }
