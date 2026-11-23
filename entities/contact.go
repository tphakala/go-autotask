package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// Contact represents an Autotask Contact entity, a customer user.
//
// Get, List and Count work at the root. The REST swagger lists no root write
// for Contacts: it offers them on the company URL, so write with
// autotask.CreateChild[Company] (POST /v1.0/Companies/{companyID}/Contacts),
// autotask.UpdateChild[Company] (PATCH to the same path, with the contact's ID
// in the body) and autotask.DeleteChild[Company, Contact] (DELETE
// /v1.0/Companies/{companyID}/Contacts/{id}). autotask.Create, autotask.Update
// and autotask.Delete send root URLs the swagger does not list for Contacts.
//
// CompanyID is required and read-only per the field metadata: set it on create
// to the id of the parent company; it cannot change afterwards. Whether the API
// needs it in the body when the URL names the company has not been checked
// against a live tenant. IsActive is an integer, 1 for active and 0 for
// inactive, unlike Resource.IsActive. SolicitationOptOut, SurveyOptOut and the
// timestamps are read-only.
//
// The fields ApiVendorID, NamePrefix, NameSuffix take a value of the field's
// picklist; list the values with metadata.GetPickList.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type Contact struct {
	AdditionalAddressInformation  autotask.Optional[string]    `json:"additionalAddressInformation,omitzero"`
	AddressLine                   autotask.Optional[string]    `json:"addressLine,omitzero"`
	AddressLine1                  autotask.Optional[string]    `json:"addressLine1,omitzero"`
	AlternatePhone                autotask.Optional[string]    `json:"alternatePhone,omitzero"`
	ApiVendorID                   autotask.Optional[int64]     `json:"apiVendorID,omitzero"`
	BillingContact                autotask.Optional[bool]      `json:"billingContact,omitzero"`
	BulkEmailOptOutTime           autotask.Optional[time.Time] `json:"bulkEmailOptOutTime,omitzero"`
	City                          autotask.Optional[string]    `json:"city,omitzero"`
	CompanyID                     autotask.Optional[int64]     `json:"companyID,omitzero"`
	CompanyLocationID             autotask.Optional[int64]     `json:"companyLocationID,omitzero"`
	CountryID                     autotask.Optional[int64]     `json:"countryID,omitzero"`
	CreateDate                    autotask.Optional[time.Time] `json:"createDate,omitzero"`
	EmailAddress                  autotask.Optional[string]    `json:"emailAddress,omitzero"`
	EmailAddress2                 autotask.Optional[string]    `json:"emailAddress2,omitzero"`
	EmailAddress3                 autotask.Optional[string]    `json:"emailAddress3,omitzero"`
	Extension                     autotask.Optional[string]    `json:"extension,omitzero"`
	ExternalID                    autotask.Optional[string]    `json:"externalID,omitzero"`
	FacebookUrl                   autotask.Optional[string]    `json:"facebookUrl,omitzero"`
	FaxNumber                     autotask.Optional[string]    `json:"faxNumber,omitzero"`
	FirstName                     autotask.Optional[string]    `json:"firstName,omitzero"`
	ID                            autotask.Optional[int64]     `json:"id,omitzero"`
	ImpersonatorCreatorResourceID autotask.Optional[int64]     `json:"impersonatorCreatorResourceID,omitzero"`
	IsActive                      autotask.Optional[int64]     `json:"isActive,omitzero"`
	IsOptedOutFromBulkEmail       autotask.Optional[bool]      `json:"isOptedOutFromBulkEmail,omitzero"`
	LastActivityDate              autotask.Optional[time.Time] `json:"lastActivityDate,omitzero"`
	LastModifiedDate              autotask.Optional[time.Time] `json:"lastModifiedDate,omitzero"`
	LastName                      autotask.Optional[string]    `json:"lastName,omitzero"`
	LinkedInUrl                   autotask.Optional[string]    `json:"linkedInUrl,omitzero"`
	MiddleInitial                 autotask.Optional[string]    `json:"middleInitial,omitzero"`
	MobilePhone                   autotask.Optional[string]    `json:"mobilePhone,omitzero"`
	NamePrefix                    autotask.Optional[int64]     `json:"namePrefix,omitzero"`
	NameSuffix                    autotask.Optional[int64]     `json:"nameSuffix,omitzero"`
	Note                          autotask.Optional[string]    `json:"note,omitzero"`
	Phone                         autotask.Optional[string]    `json:"phone,omitzero"`
	PrimaryContact                autotask.Optional[bool]      `json:"primaryContact,omitzero"`
	ReceivesEmailNotifications    autotask.Optional[bool]      `json:"receivesEmailNotifications,omitzero"`
	RoomNumber                    autotask.Optional[string]    `json:"roomNumber,omitzero"`
	SolicitationOptOut            autotask.Optional[bool]      `json:"solicitationOptOut,omitzero"`
	SolicitationOptOutTime        autotask.Optional[time.Time] `json:"solicitationOptOutTime,omitzero"`
	State                         autotask.Optional[string]    `json:"state,omitzero"`
	SurveyOptOut                  autotask.Optional[bool]      `json:"surveyOptOut,omitzero"`
	Title                         autotask.Optional[string]    `json:"title,omitzero"`
	TwitterUrl                    autotask.Optional[string]    `json:"twitterUrl,omitzero"`
	ZipCode                       autotask.Optional[string]    `json:"zipCode,omitzero"`
	UserDefinedFields             []autotask.UDF               `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for contacts.
func (Contact) EntityName() string { return "Contacts" }
