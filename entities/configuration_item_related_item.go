package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ConfigurationItemRelatedItem represents an Autotask
// ConfigurationItemRelatedItem entity, a relationship from a configuration
// item to another configuration item.
//
// Per the REST swagger, it is reached through its parent configuration item:
// autotask.ListChild[ConfigurationItem, ConfigurationItemRelatedItem] lists
// /v1.0/ConfigurationItems/{ciID}/RelatedItems,
// autotask.CreateChild[ConfigurationItem] posts to that path, and
// autotask.DeleteChild[ConfigurationItem, ConfigurationItemRelatedItem]
// sends DELETE to that path plus /{id}. The API has no update for this
// entity, so delete the relationship and create a new one. autotask.Create,
// autotask.Update and autotask.Delete send root URLs that the REST swagger
// does not list for this entity, so use the child calls. It has no
// user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not
// regenerate it.
type ConfigurationItemRelatedItem struct {
	ConfigurationItemID        autotask.Optional[int64] `json:"configurationItemID,omitzero"`
	ID                         autotask.Optional[int64] `json:"id,omitzero"`
	RelatedConfigurationItemID autotask.Optional[int64] `json:"relatedConfigurationItemID,omitzero"`
	Relationship               autotask.Optional[int64] `json:"relationship,omitzero"`
}

// EntityName returns the Autotask API entity name for configuration item related items.
func (ConfigurationItemRelatedItem) EntityName() string { return "ConfigurationItemRelatedItems" }
