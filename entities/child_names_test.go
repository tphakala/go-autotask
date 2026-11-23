package entities

import "testing"

func TestChildEntityNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"TicketNote", TicketNote{}.ChildEntityName(), "Notes"},
		{"ProjectNote", ProjectNote{}.ChildEntityName(), "Notes"},
		{"CompanyNote", CompanyNote{}.ChildEntityName(), "Notes"},
		{"TicketAttachment", TicketAttachment{}.ChildEntityName(), "Attachments"},
		{"TicketNoteAttachment", TicketNoteAttachment{}.ChildEntityName(), "Attachments"},
		{"ConfigurationItemCategoryUdfAssociation", ConfigurationItemCategoryUdfAssociation{}.ChildEntityName(), "UdfAssociations"},
		{"ConfigurationItemNote", ConfigurationItemNote{}.ChildEntityName(), "Notes"},
		{"ConfigurationItemAttachment", ConfigurationItemAttachment{}.ChildEntityName(), "Attachments"},
		{"ConfigurationItemNoteAttachment", ConfigurationItemNoteAttachment{}.ChildEntityName(), "Attachments"},
		{"ConfigurationItemRelatedItem", ConfigurationItemRelatedItem{}.ChildEntityName(), "RelatedItems"},
		{"ConfigurationItemBillingProductAssociation", ConfigurationItemBillingProductAssociation{}.ChildEntityName(), "BillingProductAssociations"},
		{"ConfigurationItemDnsRecord", ConfigurationItemDnsRecord{}.ChildEntityName(), "DnsRecords"},
		{"ConfigurationItemSslSubjectAlternativeName", ConfigurationItemSslSubjectAlternativeName{}.ChildEntityName(), "SslSubjectAlternativeNames"},
		{"TicketAdditionalConfigurationItem", TicketAdditionalConfigurationItem{}.ChildEntityName(), "AdditionalConfigurationItems"},
		{"ResourceRoleDepartment", ResourceRoleDepartment{}.ChildEntityName(), "RoleDepartments"},
		{"ResourceRoleQueue", ResourceRoleQueue{}.ChildEntityName(), "RoleQueues"},
		{"ResourceServiceDeskRole", ResourceServiceDeskRole{}.ChildEntityName(), "ServiceDeskRoles"},
		{"ResourceSkill", ResourceSkill{}.ChildEntityName(), "Skills"},
		{"ResourceRole", ResourceRole{}.ChildEntityName(), "Roles"},
		{"ResourceAttachment", ResourceAttachment{}.ChildEntityName(), "Attachments"},
		{"ContactGroupContact", ContactGroupContact{}.ChildEntityName(), "Contacts"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s.ChildEntityName() = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}
