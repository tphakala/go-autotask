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

func TestTicketProblemTicketIDWireName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		field autotask.Optional[int64]
		want  string
	}{
		{"set", autotask.Set(int64(555)), `{"problemTicketId":555}`},
		{"null", autotask.Null[int64](), `{"problemTicketId":null}`},
		{"unset", autotask.Optional[int64]{}, `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(entities.Ticket{ProblemTicketID: tt.field})
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tt.want {
				t.Fatalf("JSON = %s, want %s", data, tt.want)
			}
		})
	}
}

func TestTicketResolutionPartialUpdate(t *testing.T) {
	t.Parallel()
	ticket := entities.Ticket{
		ID:         autotask.Set(int64(1)),
		Status:     autotask.Set(int64(5)),
		Resolution: autotask.Set("Restarted the service"),
	}
	data, err := json.Marshal(ticket)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":1,"resolution":"Restarted the service","status":5}`
	if string(data) != want {
		t.Fatalf("JSON = %s, want %s", data, want)
	}
}

func TestCompanyTerritoryIDWireName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		field autotask.Optional[int64]
		want  string
	}{
		{"set", autotask.Set(int64(7)), `{"territoryID":7}`},
		{"unset", autotask.Optional[int64]{}, `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(entities.Company{TerritoryID: tt.field})
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tt.want {
				t.Fatalf("JSON = %s, want %s", data, tt.want)
			}
		})
	}
}
