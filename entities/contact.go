package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// Contact represents an Autotask Contact entity.
type Contact struct {
	ID                autotask.Optional[int64]     `json:"id,omitzero"`
	FirstName         autotask.Optional[string]    `json:"firstName,omitzero"`
	LastName          autotask.Optional[string]    `json:"lastName,omitzero"`
	Title             autotask.Optional[string]    `json:"title,omitzero"`
	EmailAddress      autotask.Optional[string]    `json:"emailAddress,omitzero"`
	EmailAddress2     autotask.Optional[string]    `json:"emailAddress2,omitzero"`
	Phone             autotask.Optional[string]    `json:"phone,omitzero"`
	MobilePhone       autotask.Optional[string]    `json:"mobilePhone,omitzero"`
	CompanyID         autotask.Optional[int64]     `json:"companyID,omitzero"`
	IsActive          autotask.Optional[int64]     `json:"isActive,omitzero"`
	CreateDate        autotask.Optional[time.Time] `json:"createDate,omitzero"`
	LastActivityDate  autotask.Optional[time.Time] `json:"lastActivityDate,omitzero"`
	UserDefinedFields []autotask.UDF               `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for contacts.
func (Contact) EntityName() string { return "Contacts" }
