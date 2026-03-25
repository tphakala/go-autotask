package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// Ticket represents an Autotask Ticket entity.
type Ticket struct {
	ID                     autotask.Optional[int64]     `json:"id,omitzero"`
	Title                  autotask.Optional[string]    `json:"title,omitzero"`
	Description            autotask.Optional[string]    `json:"description,omitzero"`
	TicketNumber           autotask.Optional[string]    `json:"ticketNumber,omitzero"`
	Status                 autotask.Optional[int64]       `json:"status,omitzero"`
	Priority               autotask.Optional[int64]       `json:"priority,omitzero"`
	QueueID                autotask.Optional[int64]     `json:"queueID,omitzero"`
	CompanyID              autotask.Optional[int64]     `json:"companyID,omitzero"`
	CompanyLocationID      autotask.Optional[int64]     `json:"companyLocationID,omitzero"`
	ContactID              autotask.Optional[int64]     `json:"contactID,omitzero"`
	ContractID             autotask.Optional[int64]     `json:"contractID,omitzero"`
	ConfigurationItemID    autotask.Optional[int64]     `json:"configurationItemID,omitzero"`
	AssignedResourceID     autotask.Optional[int64]     `json:"assignedResourceID,omitzero"`
	AssignedResourceRoleID autotask.Optional[int64]     `json:"assignedResourceRoleID,omitzero"`
	DueDateTime            autotask.Optional[time.Time] `json:"dueDateTime,omitzero"`
	CreateDate             autotask.Optional[time.Time] `json:"createDate,omitzero"`
	LastActivityDate       autotask.Optional[time.Time] `json:"lastActivityDate,omitzero"`
	CompletedDate          autotask.Optional[time.Time] `json:"completedDate,omitzero"`
	TicketType             autotask.Optional[int64]       `json:"ticketType,omitzero"`
	IssueType              autotask.Optional[int64]       `json:"issueType,omitzero"`
	SubIssueType           autotask.Optional[int64]       `json:"subIssueType,omitzero"`
	TicketCategory         autotask.Optional[int64]       `json:"ticketCategory,omitzero"`
	Source                 autotask.Optional[int64]       `json:"source,omitzero"`
	BillingCodeID          autotask.Optional[int64]     `json:"billingCodeID,omitzero"`
	EstimatedHours         autotask.Optional[float64]   `json:"estimatedHours,omitzero"`
	ExternalID             autotask.Optional[string]    `json:"externalID,omitzero"`
	LastModifiedDate       autotask.Optional[time.Time] `json:"lastModifiedDate,omitzero"`
	UserDefinedFields      []autotask.UDF               `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for tickets.
func (Ticket) EntityName() string { return "Tickets" }
