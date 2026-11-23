package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// ConfigurationItemDnsRecord represents an Autotask
// ConfigurationItemDnsRecord entity, a DNS record of a configuration item.
//
// Per the REST swagger, it is reached through its parent configuration item:
// autotask.ListChild[ConfigurationItem, ConfigurationItemDnsRecord] lists
// /v1.0/ConfigurationItems/{ciID}/DnsRecords, and
// autotask.DeleteChild[ConfigurationItem, ConfigurationItemDnsRecord] sends
// DELETE to that path plus /{id}. The API has no create or update for this
// entity (canCreate and canUpdate are false in the entity metadata). The
// field metadata names the parent configuration item InstalledProductID.
// autotask.Delete sends a root URL that the REST swagger does not list for
// this entity, so use DeleteChild. It has no user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not
// regenerate it.
type ConfigurationItemDnsRecord struct {
	CreateDateTime     autotask.Optional[time.Time] `json:"createDateTime,omitzero"`
	Data               autotask.Optional[string]    `json:"data,omitzero"`
	DnsType            autotask.Optional[string]    `json:"dnsType,omitzero"`
	ID                 autotask.Optional[int64]     `json:"id,omitzero"`
	InstalledProductID autotask.Optional[int64]     `json:"installedProductID,omitzero"`
	TimeToLiveSeconds  autotask.Optional[int64]     `json:"timeToLiveSeconds,omitzero"`
}

// EntityName returns the Autotask API entity name for configuration item DNS records.
func (ConfigurationItemDnsRecord) EntityName() string { return "ConfigurationItemDnsRecords" }
