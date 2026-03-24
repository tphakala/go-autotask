package main

import "testing"

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Tickets", "tickets"},
		{"TicketNotes", "ticket_notes"},
		{"ConfigurationItems", "configuration_items"},
		{"BillingItemApprovalLevels", "billing_item_approval_levels"},
		{"Companies", "companies"},
		{"TimeEntries", "time_entries"},
		{"ServiceBundles", "service_bundles"},
		{"QuoteItems", "quote_items"},
		{"IDs", "ids"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := toSnakeCase(tt.input); got != tt.want {
				t.Errorf("toSnakeCase(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSingular(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Tickets", "Ticket"},
		{"Companies", "Company"},
		{"TimeEntries", "TimeEntry"},
		{"ConfigurationItems", "ConfigurationItem"},
		{"Statuses", "Status"},
		{"Services", "Service"},
		{"Addresses", "Address"},
		{"Resources", "Resource"},
		{"Departments", "Department"},
		{"TicketNotes", "TicketNote"},
		{"BillingCodes", "BillingCode"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := singular(tt.input); got != tt.want {
				t.Errorf("singular(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}
