package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ConfigurationItemSslSubjectAlternativeName represents an Autotask
// ConfigurationItemSslSubjectAlternativeName entity, a subject alternative
// name of a configuration item's SSL certificate.
//
// Per the REST swagger, it is reached through its parent configuration item:
// autotask.ListChild[ConfigurationItem,
// ConfigurationItemSslSubjectAlternativeName] lists
// /v1.0/ConfigurationItems/{ciID}/SslSubjectAlternativeNames, and
// autotask.DeleteChild[ConfigurationItem,
// ConfigurationItemSslSubjectAlternativeName] sends DELETE to that path plus
// /{id}. The API has no create or update for this entity (canCreate and
// canUpdate are false in the entity metadata). autotask.Delete sends a root
// URL that the REST swagger does not list for this entity, so use
// DeleteChild. It has no user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not
// regenerate it.
type ConfigurationItemSslSubjectAlternativeName struct {
	ConfigurationItemID autotask.Optional[int64]  `json:"configurationItemID,omitzero"`
	ID                  autotask.Optional[int64]  `json:"id,omitzero"`
	Name                autotask.Optional[string] `json:"name,omitzero"`
}

// EntityName returns the Autotask API entity name for configuration item SSL subject alternative names.
func (ConfigurationItemSslSubjectAlternativeName) EntityName() string {
	return "ConfigurationItemSslSubjectAlternativeNames"
}
