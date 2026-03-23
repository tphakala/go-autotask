package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// ConfigurationItem represents an Autotask ConfigurationItem (CI) entity.
type ConfigurationItem struct {
	ID                     autotask.Optional[int64]     `json:"id,omitzero"`
	ReferenceTitle         autotask.Optional[string]    `json:"referenceTitle,omitzero"`
	ReferenceNumber        autotask.Optional[string]    `json:"referenceNumber,omitzero"`
	CompanyID              autotask.Optional[int64]     `json:"companyID,omitzero"`
	ContactID              autotask.Optional[int64]     `json:"contactID,omitzero"`
	ConfigurationItemType  autotask.Optional[int]       `json:"configurationItemType,omitzero"`
	IsActive               autotask.Optional[bool]      `json:"isActive,omitzero"`
	InstallDate            autotask.Optional[time.Time] `json:"installDate,omitzero"`
	WarrantyExpirationDate autotask.Optional[time.Time] `json:"warrantyExpirationDate,omitzero"`
	SerialNumber           autotask.Optional[string]    `json:"serialNumber,omitzero"`
	Location               autotask.Optional[string]    `json:"location,omitzero"`
	CreateDate             autotask.Optional[time.Time] `json:"createDate,omitzero"`
	LastActivityDate       autotask.Optional[time.Time] `json:"lastActivityDate,omitzero"`
	UserDefinedFields      []autotask.UDF               `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for configuration items.
func (ConfigurationItem) EntityName() string { return "ConfigurationItems" }
