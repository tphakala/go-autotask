package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// ConfigurationItemBillingProductAssociation represents an Autotask
// ConfigurationItemBillingProductAssociation entity, the link between a
// configuration item and a billing product.
//
// Per the REST swagger, it is reached through its parent configuration item:
// autotask.ListChild[ConfigurationItem,
// ConfigurationItemBillingProductAssociation] lists
// /v1.0/ConfigurationItems/{ciID}/BillingProductAssociations, and
// autotask.CreateChild and autotask.UpdateChild[ConfigurationItem] write to
// that path (UpdateChild carries the association's ID in the body), while
// autotask.DeleteChild[ConfigurationItem,
// ConfigurationItemBillingProductAssociation] sends DELETE to that path plus
// /{id}. autotask.Create, autotask.Update and autotask.Delete send root URLs
// that the REST swagger does not list for this entity, so use the child
// calls. It has no user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not
// regenerate it.
type ConfigurationItemBillingProductAssociation struct {
	BillingProductID    autotask.Optional[int64]     `json:"billingProductID,omitzero"`
	ConfigurationItemID autotask.Optional[int64]     `json:"configurationItemID,omitzero"`
	EffectiveDate       autotask.Optional[time.Time] `json:"effectiveDate,omitzero"`
	ExpirationDate      autotask.Optional[time.Time] `json:"expirationDate,omitzero"`
	ID                  autotask.Optional[int64]     `json:"id,omitzero"`
}

// EntityName returns the Autotask API entity name for configuration item billing product associations.
func (ConfigurationItemBillingProductAssociation) EntityName() string {
	return "ConfigurationItemBillingProductAssociations"
}
