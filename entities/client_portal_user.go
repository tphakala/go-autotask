package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ClientPortalUser represents an Autotask ClientPortalUser entity, the client
// portal access of a contact.
//
// It is a root entity: Get, List and Count work, autotask.Create sends POST to
// /v1.0/ClientPortalUsers and autotask.Update sends PATCH to it per the REST
// swagger. The API has no delete (canDelete is false in the entity information
// and the swagger lists no DELETE); set IsClientPortalActive to false to
// disable access (not checked against a live tenant). ContactID is read-only
// per the field metadata: set it on create to the id of the contact, and it
// cannot change afterwards. The picklist fields here are integers, unlike the
// string date, number and time formats of Resource.
//
// Password is not queryable and not read-only per the field metadata. Whether
// the API ever returns it has not been checked against a live tenant. Update
// sends every field that is set, so set Password only to change it. It has no
// user-defined fields.
//
// The fields DateFormat, NumberFormat, SecurityLevel, TimeFormat take a value
// of the field's picklist; list the values with metadata.GetPickList.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type ClientPortalUser struct {
	ContactID            autotask.Optional[int64]  `json:"contactID,omitzero"`
	DateFormat           autotask.Optional[int64]  `json:"dateFormat,omitzero"`
	ID                   autotask.Optional[int64]  `json:"id,omitzero"`
	IsClientPortalActive autotask.Optional[bool]   `json:"isClientPortalActive,omitzero"`
	NumberFormat         autotask.Optional[int64]  `json:"numberFormat,omitzero"`
	Password             autotask.Optional[string] `json:"password,omitzero"`
	SecurityLevel        autotask.Optional[int64]  `json:"securityLevel,omitzero"`
	TimeFormat           autotask.Optional[int64]  `json:"timeFormat,omitzero"`
	UserName             autotask.Optional[string] `json:"userName,omitzero"`
}

// EntityName returns the Autotask API entity name for client portal users.
func (ClientPortalUser) EntityName() string { return "ClientPortalUsers" }
