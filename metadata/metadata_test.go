package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	autotask "github.com/tphakala/go-autotask"
)

func testClient(t *testing.T, handler http.Handler) *autotask.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	auth := autotask.AuthConfig{Username: "u", Secret: "s", IntegrationCode: "c"}
	client, err := autotask.NewClient(context.Background(), auth, autotask.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestGetFields(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"fields": []any{
				map[string]any{
					"name": "status", "label": "Status", "dataType": "integer",
					"isRequired": true, "isReadOnly": false, "isPickList": true,
					"picklistValues": []any{
						map[string]any{"value": 1, "label": "New", "isActive": true},
						map[string]any{"value": 5, "label": "Complete", "isActive": true},
					},
				},
			},
		})
	})
	client := testClient(t, handler)
	fields, err := GetFields(context.Background(), client, "Tickets")
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 {
		t.Fatalf("fields = %d; want 1", len(fields))
	}
	if fields[0].Name != "status" {
		t.Fatalf("name = %q; want status", fields[0].Name)
	}
	if len(fields[0].PickListValues) != 2 {
		t.Fatalf("picklist = %d; want 2", len(fields[0].PickListValues))
	}
}

func TestGetUDFs(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"fields": []any{
				map[string]any{
					"name": "CustomField1", "label": "Custom Field 1",
					"dataType": "string", "isRequired": false,
				},
			},
		})
	})
	client := testClient(t, handler)
	udfs, err := GetUDFs(context.Background(), client, "Tickets")
	if err != nil {
		t.Fatal(err)
	}
	if len(udfs) != 1 {
		t.Fatalf("udfs = %d; want 1", len(udfs))
	}
	if udfs[0].Name != "CustomField1" {
		t.Fatalf("name = %q; want CustomField1", udfs[0].Name)
	}
}

func TestGetEntityInfo(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"name": "Tickets", "canCreate": true, "canUpdate": true,
			"canDelete": false, "canQuery": true, "hasUserDefinedFields": true,
		})
	})
	client := testClient(t, handler)
	info, err := GetEntityInfo(context.Background(), client, "Tickets")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Tickets" {
		t.Fatalf("name = %q", info.Name)
	}
	if !info.CanCreate {
		t.Fatal("expected CanCreate=true")
	}
}
