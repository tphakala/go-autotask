package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// Company represents an Autotask Company entity.
type Company struct {
	ID               autotask.Optional[int64]     `json:"id,omitzero"`
	CompanyName      autotask.Optional[string]    `json:"companyName,omitzero"`
	CompanyNumber    autotask.Optional[string]    `json:"companyNumber,omitzero"`
	Phone            autotask.Optional[string]    `json:"phone,omitzero"`
	Fax              autotask.Optional[string]    `json:"fax,omitzero"`
	WebAddress       autotask.Optional[string]    `json:"webAddress,omitzero"`
	Address1         autotask.Optional[string]    `json:"address1,omitzero"`
	Address2         autotask.Optional[string]    `json:"address2,omitzero"`
	City             autotask.Optional[string]    `json:"city,omitzero"`
	State            autotask.Optional[string]    `json:"state,omitzero"`
	PostalCode       autotask.Optional[string]    `json:"postalCode,omitzero"`
	Country          autotask.Optional[string]    `json:"country,omitzero"`
	CompanyType      autotask.Optional[int64]       `json:"companyType,omitzero"`
	Classification   autotask.Optional[int64]       `json:"classification,omitzero"`
	OwnerResourceID  autotask.Optional[int64]     `json:"ownerResourceID,omitzero"`
	IsActive         autotask.Optional[bool]      `json:"isActive,omitzero"`
	CreateDate       autotask.Optional[time.Time] `json:"createDate,omitzero"`
	LastActivityDate autotask.Optional[time.Time] `json:"lastActivityDate,omitzero"`
	UserDefinedFields []autotask.UDF              `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for companies.
func (Company) EntityName() string { return "Companies" }
