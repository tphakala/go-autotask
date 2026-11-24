package metadata

import (
	"context"
	"encoding/json"
	"errors"
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
	client, err := autotask.NewClient(t.Context(), auth, autotask.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestGetFields(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.0/Tickets/entityInformation/fields" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test handler, encoding any is intentional
			"fields": []any{
				map[string]any{
					"name": "status", "label": "Status", "dataType": "integer",
					"isRequired": true, "isReadOnly": false, "isPickList": true,
					"picklistValues": []any{
						map[string]any{"value": "1", "label": "New", "isActive": true},
						map[string]any{"value": "5", "label": "Complete", "isActive": true},
					},
				},
			},
		})
	})
	client := testClient(t, handler)
	fields, err := GetFields(t.Context(), client, "Tickets")
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
		_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test handler, encoding any is intentional
			"fields": []any{
				map[string]any{
					"name": "CustomField1", "label": "Custom Field 1",
					"dataType": "string", "isRequired": false,
				},
			},
		})
	})
	client := testClient(t, handler)
	udfs, err := GetUDFs(t.Context(), client, "Tickets")
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
		_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test handler, encoding any is intentional
			"info": map[string]any{
				"name": "Tickets", "canCreate": true, "canUpdate": true,
				"canDelete": false, "canQuery": true, "hasUserDefinedFields": true,
			},
		})
	})
	client := testClient(t, handler)
	info, err := GetEntityInfo(t.Context(), client, "Tickets")
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

func TestGetPickList(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test handler, encoding any is intentional
			"fields": []any{
				map[string]any{
					"name": "title", "label": "Title", "dataType": "string",
					"isRequired": true, "isReadOnly": false, "isPickList": false,
				},
				map[string]any{
					"name": "status", "label": "Status", "dataType": "integer",
					"isRequired": true, "isReadOnly": false, "isPickList": true,
					"picklistValues": []any{
						map[string]any{"value": "1", "label": "New", "isActive": true},
						map[string]any{"value": "5", "label": "Complete", "isActive": true},
					},
				},
			},
		})
	})
	client := testClient(t, handler)

	t.Run("valid picklist field", func(t *testing.T) {
		values, err := GetPickList(t.Context(), client, "Tickets", "status")
		if err != nil {
			t.Fatal(err)
		}
		if len(values) != 2 {
			t.Fatalf("values = %d; want 2", len(values))
		}
		if values[0].Label != "New" {
			t.Fatalf("label = %q; want New", values[0].Label)
		}
	})

	t.Run("non-picklist field", func(t *testing.T) {
		_, err := GetPickList(t.Context(), client, "Tickets", "title")
		if err == nil {
			t.Fatal("expected error for non-picklist field")
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		_, err := GetPickList(t.Context(), client, "Tickets", "nonexistent")
		if err == nil {
			t.Fatal("expected error for unknown field")
		}
	})
}

// bodyClient serves body with status 200 and a JSON content type for every
// request.
func bodyClient(t *testing.T, body string) *autotask.Client {
	t.Helper()
	return testClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

// resultCall is one metadata call reduced to the size of what it returned and
// its error. The size is len for a list and 0 or 1 for a nil or non-nil
// EntityInfo, so a typed nil is not boxed in an interface.
type resultCall func(context.Context, *autotask.Client) (int, error)

func getFields(ctx context.Context, c *autotask.Client) (int, error) {
	fields, err := GetFields(ctx, c, "Tickets")
	return len(fields), err
}

func getUDFs(ctx context.Context, c *autotask.Client) (int, error) {
	udfs, err := GetUDFs(ctx, c, "Tickets")
	return len(udfs), err
}

func getEntityInfo(ctx context.Context, c *autotask.Client) (int, error) {
	info, err := GetEntityInfo(ctx, c, "Tickets")
	if info == nil {
		return 0, err
	}
	return 1, err
}

func TestMetadataResponseWithoutResultIsMissingMemberError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		call   resultCall
		body   string
		member string
	}{
		{"GetFields", getFields, `{}`, "fields"},
		{"GetUDFs", getUDFs, `{}`, "fields"},
		{"GetEntityInfo", getEntityInfo, `{}`, "info"},
		{"GetEntityInfo null info", getEntityInfo, `{"info":null}`, "info"},
	}
	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.body, func(t *testing.T) {
			t.Parallel()
			n, err := tt.call(t.Context(), bodyClient(t, tt.body))
			mm, ok := errors.AsType[*autotask.MissingMemberError](err)
			if !ok {
				t.Fatalf("err = %T %v; want *autotask.MissingMemberError", err, err)
			}
			if mm.Member != tt.member {
				t.Errorf("Member = %q; want %q", mm.Member, tt.member)
			}
			if mm.Err.StatusCode != http.StatusOK {
				t.Errorf("Err.StatusCode = %d; want 200", mm.Err.StatusCode)
			}
			if _, ok := errors.AsType[*autotask.Error](err); !ok {
				t.Errorf("errors.As(%T, *autotask.Error) = false; want true", err)
			}
			if _, ok := errors.AsType[*autotask.NotFoundError](err); ok {
				t.Errorf("%T matches *autotask.NotFoundError; a response without its member is not a missing record", err)
			}
			if !autotask.IsTransient(err) {
				t.Errorf("IsTransient(%v) = false; want true", err)
			}
			if n != 0 {
				t.Errorf("returned %d results beside the error; want 0", n)
			}
		})
	}
}

func TestMetadataResponseWithPresentEmptyMemberIsAResult(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		call resultCall
		body string
		want int
	}{
		{"GetFields", getFields, `{"fields":[]}`, 0},
		{"GetFields", getFields, `{"fields":null}`, 0},
		{"GetUDFs", getUDFs, `{"fields":[]}`, 0},
		{"GetUDFs", getUDFs, `{"fields":null}`, 0},
		{"GetEntityInfo", getEntityInfo, `{"info":{}}`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.body, func(t *testing.T) {
			t.Parallel()
			n, err := tt.call(t.Context(), bodyClient(t, tt.body))
			if err != nil {
				t.Fatalf("err = %v; want nil", err)
			}
			if n != tt.want {
				t.Errorf("result size = %d; want %d", n, tt.want)
			}
		})
	}
}
