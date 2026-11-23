package autotasktest

import (
	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
)

// knownEntityNames holds the entity name of every entity type in package
// entities. resolveChildStore uses it to route a child URL of one of these
// types by name, whatever other stores exist. TestKnownEntityNamesCoverEntities
// keeps the list complete.
var knownEntityNames = entityNameSet(
	entities.BillingCode{}, entities.BillingItemApprovalLevel{}, entities.BillingItem{}, entities.ChangeRequestLink{},
	entities.CompanyNote{}, entities.Company{}, entities.ConfigurationItem{}, entities.ConfigurationItemAttachment{},
	entities.ConfigurationItemBillingProductAssociation{}, entities.ConfigurationItemCategoryUdfAssociation{},
	entities.ConfigurationItemCategory{}, entities.ConfigurationItemDnsRecord{}, entities.ConfigurationItemNoteAttachment{},
	entities.ConfigurationItemNote{}, entities.ConfigurationItemRelatedItem{}, entities.ConfigurationItemSslSubjectAlternativeName{},
	entities.ConfigurationItemType{}, entities.TicketAdditionalConfigurationItem{}, entities.Contact{}, entities.Contract{},
	entities.Department{}, entities.ExpenseItem{}, entities.ExpenseReport{}, entities.Invoice{}, entities.Opportunity{},
	entities.Product{}, entities.ProjectNote{}, entities.Project{}, entities.QuoteItem{}, entities.Quote{}, entities.Resource{},
	entities.ServiceBundle{}, entities.Service{}, entities.Task{}, entities.TicketAttachment{}, entities.TicketNoteAttachment{},
	entities.TicketNote{}, entities.Ticket{}, entities.TimeEntry{},
)

// entityNameSet returns the set of EntityName values of items.
func entityNameSet(items ...autotask.Entity) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it.EntityName()] = true
	}
	return m
}
