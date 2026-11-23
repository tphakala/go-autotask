package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ConfigurationItemCategoryUdfAssociation represents an Autotask
// ConfigurationItemCategoryUdfAssociation entity, the link between a
// configuration item category and a user-defined field definition.
//
// Per the REST swagger, it is reached through its parent category:
// autotask.ListChild[ConfigurationItemCategory,
// ConfigurationItemCategoryUdfAssociation] lists
// /v1.0/ConfigurationItemCategories/{categoryID}/UdfAssociations, and
// autotask.CreateChild and autotask.UpdateChild[ConfigurationItemCategory]
// write to that path (UpdateChild carries the association's ID in the body),
// while autotask.DeleteChild[ConfigurationItemCategory,
// ConfigurationItemCategoryUdfAssociation] sends DELETE to that path plus
// /{id}. autotask.Create, autotask.Update and autotask.Delete send root URLs
// that the REST swagger does not list for this entity, so use the child
// calls. It has no user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not
// regenerate it.
type ConfigurationItemCategoryUdfAssociation struct {
	ConfigurationItemCategoryID  autotask.Optional[int64] `json:"configurationItemCategoryID,omitzero"`
	ID                           autotask.Optional[int64] `json:"id,omitzero"`
	IsRequired                   autotask.Optional[bool]  `json:"isRequired,omitzero"`
	UserDefinedFieldDefinitionID autotask.Optional[int64] `json:"userDefinedFieldDefinitionID,omitzero"`
}

// EntityName returns the Autotask API entity name for configuration item category UDF associations.
func (ConfigurationItemCategoryUdfAssociation) EntityName() string {
	return "ConfigurationItemCategoryUdfAssociations"
}
