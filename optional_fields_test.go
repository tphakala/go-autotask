package autotask_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/entities"
)

func TestOptionalSetSerialization(t *testing.T) {
	t.Parallel()
	company := entities.Company{
		CompanyName: autotask.Set("Test Corp"),
		CompanyType: autotask.Set(int64(1)),
	}
	data, err := json.Marshal(company)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"companyName":"Test Corp"`) {
		t.Fatalf("JSON = %s, missing companyName", data)
	}
}

func TestOptionalNullSerialization(t *testing.T) {
	t.Parallel()
	company := entities.Company{
		CompanyName: autotask.Set("Test"),
		Phone:       autotask.Null[string](),
	}
	data, err := json.Marshal(company)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"phone":null`) {
		t.Fatalf("JSON = %s, missing phone:null", data)
	}
}

func TestOptionalUnsetOmitted(t *testing.T) {
	t.Parallel()
	company := entities.Company{
		CompanyName: autotask.Set("Test"),
	}
	data, err := json.Marshal(company)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"phone"`) {
		t.Fatalf("JSON = %s, should not contain phone (unset)", data)
	}
}

func TestOptionalRoundTrip(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(company))

	id, _ := company.ID.Get()
	got, err := autotask.Get[entities.Company](t.Context(), client, id)
	if err != nil {
		t.Fatal(err)
	}

	gotName, _ := got.CompanyName.Get()
	wantName, _ := company.CompanyName.Get()
	if gotName != wantName {
		t.Fatalf("CompanyName = %q, want %q", gotName, wantName)
	}
	if !got.IsActive.IsSet() {
		t.Fatal("IsActive should be set after round-trip")
	}
}

func TestOptionalIntegerSerialization(t *testing.T) {
	t.Parallel()
	o := autotask.Set(int64(42))
	data, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "42" {
		t.Fatalf("Marshal Set(int64(42)) = %s; want 42", data)
	}
}

func TestOptionalTimeSerialization(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 3, 24, 10, 30, 0, 0, time.UTC)
	o := autotask.Set(ts)
	data, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	// time.Time marshals to RFC3339 (ISO 8601) format by default.
	want := `"2026-03-24T10:30:00Z"`
	if string(data) != want {
		t.Fatalf("Marshal Set(time.Time) = %s; want %s", data, want)
	}
}
