package entities

import (
	"encoding/json"
	"testing"

	autotask "github.com/tphakala/go-autotask"
)

func TestChangeRequestLinkWireShape(t *testing.T) {
	t.Parallel()
	if got := (ChangeRequestLink{}).EntityName(); got != "ChangeRequestLinks" {
		t.Fatalf("EntityName = %q, want ChangeRequestLinks", got)
	}
	tests := []struct {
		name string
		link ChangeRequestLink
		want string
	}{
		{"all set", ChangeRequestLink{
			ID:                        autotask.Set(int64(1)),
			ChangeRequestTicketID:     autotask.Set(int64(2)),
			ProblemOrIncidentTicketID: autotask.Set(int64(3)),
		}, `{"id":1,"changeRequestTicketID":2,"problemOrIncidentTicketID":3}`},
		{"id unset on create", ChangeRequestLink{
			ChangeRequestTicketID:     autotask.Set(int64(2)),
			ProblemOrIncidentTicketID: autotask.Set(int64(3)),
		}, `{"changeRequestTicketID":2,"problemOrIncidentTicketID":3}`},
		{"empty", ChangeRequestLink{}, `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(tt.link)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tt.want {
				t.Fatalf("JSON = %s, want %s", data, tt.want)
			}
		})
	}
}
