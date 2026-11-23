package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// Resource represents an Autotask Resource entity, an internal user.
//
// The API offers Get, List, Count and Update for it: autotask.Update sends
// PATCH to /v1.0/Resources per the REST swagger, which lists no create or
// delete at the root, and the entity information reports canCreate and
// canDelete as false. Users are added in Autotask, not through the API. To
// deactivate a user, update it with IsActive false (not checked against a live
// tenant). DateFormat, HireDate, InternalCost, LicenseType, LocationID,
// SurveyResourceRating, TimeFormat and UserType are read-only per the field
// metadata, and Update sends every field that is set, so to change a few fields
// send a value that carries ID and only those fields.
//
// Its child entities are reached through the parent URL with the child calls:
// ResourceRoleDepartment, ResourceRoleQueue and ResourceServiceDeskRole
// (ListChild, CreateChild, UpdateChild), ResourceSkill (ListChild,
// UpdateChild), ResourceAttachment (ListChild, CreateChild, DeleteChild) and
// ResourceRole (ListChild).
//
// IsActive is a bool here, unlike Contact.IsActive, which is an integer.
// UserDefinedFields is kept for source compatibility; the REST swagger lists no
// user-defined fields for Resources.
//
// The fields DateFormat, EmailTypeCode, EmailTypeCode2, EmailTypeCode3, Gender,
// Greeting, LicenseType, LocationID, NumberFormat, PayrollType, ResourceType,
// Suffix, TimeFormat, TravelAvailabilityPct, UserType take a value of the
// field's picklist; list the values with metadata.GetPickList.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type Resource struct {
	AccountingReferenceID    autotask.Optional[string]    `json:"accountingReferenceID,omitzero"`
	DateFormat               autotask.Optional[string]    `json:"dateFormat,omitzero"`
	DefaultServiceDeskRoleID autotask.Optional[int64]     `json:"defaultServiceDeskRoleID,omitzero"`
	Email                    autotask.Optional[string]    `json:"email,omitzero"`
	Email2                   autotask.Optional[string]    `json:"email2,omitzero"`
	Email3                   autotask.Optional[string]    `json:"email3,omitzero"`
	EmailTypeCode            autotask.Optional[string]    `json:"emailTypeCode,omitzero"`
	EmailTypeCode2           autotask.Optional[string]    `json:"emailTypeCode2,omitzero"`
	EmailTypeCode3           autotask.Optional[string]    `json:"emailTypeCode3,omitzero"`
	FirstName                autotask.Optional[string]    `json:"firstName,omitzero"`
	Gender                   autotask.Optional[string]    `json:"gender,omitzero"`
	Greeting                 autotask.Optional[int64]     `json:"greeting,omitzero"`
	HireDate                 autotask.Optional[time.Time] `json:"hireDate,omitzero"`
	HomePhone                autotask.Optional[string]    `json:"homePhone,omitzero"`
	ID                       autotask.Optional[int64]     `json:"id,omitzero"`
	Initials                 autotask.Optional[string]    `json:"initials,omitzero"`
	InternalCost             autotask.Optional[float64]   `json:"internalCost,omitzero"`
	IsActive                 autotask.Optional[bool]      `json:"isActive,omitzero"`
	LastName                 autotask.Optional[string]    `json:"lastName,omitzero"`
	LicenseType              autotask.Optional[int64]     `json:"licenseType,omitzero"`
	LocationID               autotask.Optional[int64]     `json:"locationID,omitzero"`
	MiddleName               autotask.Optional[string]    `json:"middleName,omitzero"`
	MobilePhone              autotask.Optional[string]    `json:"mobilePhone,omitzero"`
	NumberFormat             autotask.Optional[string]    `json:"numberFormat,omitzero"`
	OfficeExtension          autotask.Optional[string]    `json:"officeExtension,omitzero"`
	OfficePhone              autotask.Optional[string]    `json:"officePhone,omitzero"`
	PayrollIdentifier        autotask.Optional[string]    `json:"payrollIdentifier,omitzero"`
	PayrollType              autotask.Optional[int64]     `json:"payrollType,omitzero"`
	ResourceType             autotask.Optional[string]    `json:"resourceType,omitzero"`
	Suffix                   autotask.Optional[int64]     `json:"suffix,omitzero"`
	SurveyResourceRating     autotask.Optional[float64]   `json:"surveyResourceRating,omitzero"`
	TimeFormat               autotask.Optional[string]    `json:"timeFormat,omitzero"`
	Title                    autotask.Optional[string]    `json:"title,omitzero"`
	TravelAvailabilityPct    autotask.Optional[string]    `json:"travelAvailabilityPct,omitzero"`
	UserName                 autotask.Optional[string]    `json:"userName,omitzero"`
	UserType                 autotask.Optional[int64]     `json:"userType,omitzero"`
	UserDefinedFields        []autotask.UDF               `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for resources.
func (Resource) EntityName() string { return "Resources" }
