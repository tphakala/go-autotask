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
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s.ChildEntityName() = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}
