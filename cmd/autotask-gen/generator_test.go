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
		{"HTTPServer", "http_server"},
		{"APIKey", "api_key"},
		{"HTMLParser", "html_parser"},
		{"SimpleURL", "simple_url"},
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

func TestGoName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		// Basic uppercasing
		{"title", "Title"},
		{"companyName", "CompanyName"},
		// Standalone acronyms
		{"id", "ID"},
		{"url", "URL"},
		{"api", "API"},
		{"html", "HTML"},
		{"sql", "SQL"},
		{"ip", "IP"},
		{"ssl", "SSL"},
		{"cpu", "CPU"},
		{"sku", "SKU"},
		// Compound fields with acronyms already correct from API
		{"companyID", "CompanyID"},
		{"ticketID", "TicketID"},
		{"nextPageUrl", "NextPageUrl"},
		// Already uppercase acronyms pass through
		{"URL", "URL"},
		{"ID", "ID"},
		// Edge cases
		{"", ""},
		{"a", "A"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := goName(tt.input); got != tt.want {
				t.Errorf("goName(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}
