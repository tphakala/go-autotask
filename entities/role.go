package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// Role represents an Autotask Role entity, a role that can be assigned to
// resources.
//
// It is a root entity: Get, List and Count work, autotask.Create sends POST to
// /v1.0/Roles and autotask.Update sends PATCH to it per the REST swagger. The
// API has no delete (canDelete is false in the entity information and the
// swagger lists no DELETE); set IsActive to false to retire a role (not checked
// against a live tenant). It has no user-defined fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type Role struct {
	Description                   autotask.Optional[string]  `json:"description,omitzero"`
	HourlyFactor                  autotask.Optional[float64] `json:"hourlyFactor,omitzero"`
	HourlyRate                    autotask.Optional[float64] `json:"hourlyRate,omitzero"`
	ID                            autotask.Optional[int64]   `json:"id,omitzero"`
	IsActive                      autotask.Optional[bool]    `json:"isActive,omitzero"`
	IsExcludedFromNewContracts    autotask.Optional[bool]    `json:"isExcludedFromNewContracts,omitzero"`
	IsSystemRole                  autotask.Optional[bool]    `json:"isSystemRole,omitzero"`
	Name                          autotask.Optional[string]  `json:"name,omitzero"`
	QuoteItemDefaultTaxCategoryID autotask.Optional[int64]   `json:"quoteItemDefaultTaxCategoryId,omitzero"`
	RoleType                      autotask.Optional[int64]   `json:"roleType,omitzero"`
}

// EntityName returns the Autotask API entity name for roles.
func (Role) EntityName() string { return "Roles" }
