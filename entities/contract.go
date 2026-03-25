package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// Contract represents an Autotask Contract entity.
type Contract struct {
	ID                autotask.Optional[int64]     `json:"id,omitzero"`
	ContractName      autotask.Optional[string]    `json:"contractName,omitzero"`
	Description       autotask.Optional[string]    `json:"description,omitzero"`
	CompanyID         autotask.Optional[int64]     `json:"companyID,omitzero"`
	ContactID         autotask.Optional[int64]     `json:"contactID,omitzero"`
	Status            autotask.Optional[int64]       `json:"status,omitzero"`
	ContractType      autotask.Optional[int64]       `json:"contractType,omitzero"`
	StartDate         autotask.Optional[time.Time] `json:"startDate,omitzero"`
	EndDate           autotask.Optional[time.Time] `json:"endDate,omitzero"`
	EstimatedHours    autotask.Optional[float64]   `json:"estimatedHours,omitzero"`
	EstimatedRevenue  autotask.Optional[float64]   `json:"estimatedRevenue,omitzero"`
	SetupFee          autotask.Optional[float64]   `json:"setupFee,omitzero"`
	IsDefaultContract autotask.Optional[bool]      `json:"isDefaultContract,omitzero"`
	UserDefinedFields []autotask.UDF               `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for contracts.
func (Contract) EntityName() string { return "Contracts" }
