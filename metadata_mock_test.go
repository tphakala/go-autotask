package autotask_test

import (
	"testing"

	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/metadata"
)

func TestMetadataGetEntityInfo(t *testing.T) {
	t.Parallel()
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntityMetadata("Companies",
			autotasktest.EntityInfoResponse{
				Name:                 "Companies",
				CanCreate:            true,
				CanUpdate:            true,
				CanDelete:            false,
				CanQuery:             true,
				HasUserDefinedFields: true,
			},
			nil,
			nil,
		),
	)

	info, err := metadata.GetEntityInfo(t.Context(), client, "Companies")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Companies" {
		t.Fatalf("Name = %q; want Companies", info.Name)
	}
	if !info.CanCreate {
		t.Fatal("expected CanCreate = true")
	}
	if !info.CanUpdate {
		t.Fatal("expected CanUpdate = true")
	}
	if info.CanDelete {
		t.Fatal("expected CanDelete = false")
	}
	if !info.CanQuery {
		t.Fatal("expected CanQuery = true")
	}
	if !info.HasUserDefinedFields {
		t.Fatal("expected HasUserDefinedFields = true")
	}
}

func TestMetadataGetFields(t *testing.T) {
	t.Parallel()
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntityMetadata("Companies",
			autotasktest.EntityInfoResponse{Name: "Companies"},
			[]autotasktest.FieldInfoResponse{
				{Name: "id", DataType: "integer", IsRequired: true, IsReadOnly: true},
				{Name: "companyName", DataType: "string", IsRequired: true},
				{Name: "phone", DataType: "string"},
			},
			nil,
		),
	)

	fields, err := metadata.GetFields(t.Context(), client, "Companies")
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 3 {
		t.Fatalf("got %d fields, want 3", len(fields))
	}
	if fields[0].Name != "id" {
		t.Fatalf("fields[0].Name = %q; want id", fields[0].Name)
	}
	if fields[0].Type != "integer" {
		t.Fatalf("fields[0].Type = %q; want integer", fields[0].Type)
	}
	if !fields[0].IsRequired {
		t.Fatal("id should be required")
	}
	if !fields[0].IsReadOnly {
		t.Fatal("id should be read-only")
	}
	if fields[1].Name != "companyName" { //nolint:goconst // test assertion
		t.Fatalf("fields[1].Name = %q; want companyName", fields[1].Name)
	}
	if !fields[1].IsRequired {
		t.Fatal("companyName should be required")
	}
	if fields[2].Name != "phone" {
		t.Fatalf("fields[2].Name = %q; want phone", fields[2].Name)
	}
	if fields[2].IsRequired {
		t.Fatal("phone should not be required")
	}
}

func TestMetadataGetUDFs(t *testing.T) {
	t.Parallel()
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntityMetadata("Companies",
			autotasktest.EntityInfoResponse{Name: "Companies"},
			nil,
			[]autotasktest.UDFInfoResponse{
				{Name: "CustomField1", Label: "Custom Field 1", DataType: "string", IsRequired: false},
				{Name: "CustomField2", Label: "Custom Field 2", DataType: "integer", IsRequired: true},
			},
		),
	)

	udfs, err := metadata.GetUDFs(t.Context(), client, "Companies")
	if err != nil {
		t.Fatal(err)
	}
	if len(udfs) != 2 {
		t.Fatalf("got %d UDFs, want 2", len(udfs))
	}
	if udfs[0].Name != "CustomField1" {
		t.Fatalf("udfs[0].Name = %q; want CustomField1", udfs[0].Name)
	}
	if udfs[0].Type != "string" {
		t.Fatalf("udfs[0].Type = %q; want string", udfs[0].Type)
	}
	if udfs[0].IsRequired {
		t.Fatal("CustomField1 should not be required")
	}
	if udfs[1].Name != "CustomField2" {
		t.Fatalf("udfs[1].Name = %q; want CustomField2", udfs[1].Name)
	}
	if udfs[1].Type != "integer" {
		t.Fatalf("udfs[1].Type = %q; want integer", udfs[1].Type)
	}
	if !udfs[1].IsRequired {
		t.Fatal("CustomField2 should be required")
	}
}

func TestMetadataDefaultResponse(t *testing.T) {
	t.Parallel()
	// No metadata configured — the mock server should return a default response.
	_, client := autotasktest.NewServer(t)

	info, err := metadata.GetEntityInfo(t.Context(), client, "Tickets")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Tickets" {
		t.Fatalf("Name = %q; want Tickets", info.Name)
	}
	if !info.CanCreate {
		t.Fatal("expected default CanCreate = true")
	}
	if !info.CanUpdate {
		t.Fatal("expected default CanUpdate = true")
	}
	if !info.CanQuery {
		t.Fatal("expected default CanQuery = true")
	}
	if info.CanDelete {
		t.Fatal("expected default CanDelete = false")
	}
	if info.HasUserDefinedFields {
		t.Fatal("expected default HasUserDefinedFields = false")
	}
}
