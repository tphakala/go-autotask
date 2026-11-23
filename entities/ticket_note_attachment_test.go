package entities

import (
	"encoding/json"
	"testing"
	"time"

	autotask "github.com/tphakala/go-autotask"
)

func TestTicketNoteAttachmentWireShape(t *testing.T) {
	t.Parallel()
	if got := (TicketNoteAttachment{}).EntityName(); got != "TicketNoteAttachments" {
		t.Fatalf("EntityName = %q, want TicketNoteAttachments", got)
	}
	tests := []struct {
		name string
		att  TicketNoteAttachment
		want string
	}{
		{"all set", TicketNoteAttachment{
			AttachDate:                    autotask.Set(time.Date(2026, 11, 23, 8, 0, 0, 0, time.UTC)),
			AttachedByContactID:           autotask.Set(int64(1)),
			AttachedByResourceID:          autotask.Set(int64(2)),
			AttachmentType:                autotask.Set("FILE_ATTACHMENT"),
			ContentType:                   autotask.Set("application/pdf"),
			CreatorType:                   autotask.Set(int64(3)),
			Data:                          autotask.Set("AAAA"),
			FileSize:                      autotask.Set(int64(4)),
			FullPath:                      autotask.Set("report.pdf"),
			ID:                            autotask.Set(int64(5)),
			ImpersonatorCreatorResourceID: autotask.Set(int64(6)),
			OpportunityID:                 autotask.Set(int64(7)),
			ParentID:                      autotask.Set(int64(8)),
			Publish:                       autotask.Set(int64(1)),
			TicketID:                      autotask.Set(int64(9)),
			TicketNoteID:                  autotask.Set(int64(10)),
			Title:                         autotask.Set("report"),
		}, `{"attachDate":"2026-11-23T08:00:00Z","attachedByContactID":1,"attachedByResourceID":2,` +
			`"attachmentType":"FILE_ATTACHMENT","contentType":"application/pdf","creatorType":3,"data":"AAAA",` +
			`"fileSize":4,"fullPath":"report.pdf","id":5,"impersonatorCreatorResourceID":6,"opportunityID":7,` +
			`"parentID":8,"publish":1,"ticketID":9,"ticketNoteID":10,"title":"report"}`},
		{"create fields only", TicketNoteAttachment{
			AttachmentType: autotask.Set("FILE_ATTACHMENT"),
			Data:           autotask.Set("AAAA"),
			FullPath:       autotask.Set("report.pdf"),
			Publish:        autotask.Set(int64(1)),
			Title:          autotask.Set("report"),
		}, `{"attachmentType":"FILE_ATTACHMENT","data":"AAAA","fullPath":"report.pdf","publish":1,"title":"report"}`},
		{"empty", TicketNoteAttachment{}, `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(tt.att)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tt.want {
				t.Fatalf("JSON = %s, want %s", data, tt.want)
			}
		})
	}
	// The entity has no user-defined fields, so a userDefinedFields member in
	// a response is not kept and is not sent back.
	t.Run("userDefinedFields not kept", func(t *testing.T) {
		t.Parallel()
		var att TicketNoteAttachment
		in := `{"title":"report","userDefinedFields":[{"name":"x","value":"y"}]}`
		if err := json.Unmarshal([]byte(in), &att); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(att)
		if err != nil {
			t.Fatal(err)
		}
		if want := `{"title":"report"}`; string(data) != want {
			t.Fatalf("JSON = %s, want %s", data, want)
		}
	})
}
