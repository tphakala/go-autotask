package autotasktest

import (
	"sync/atomic"
	"time"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
)

var fixtureIDCounter atomic.Int64

func init() {
	fixtureIDCounter.Store(1000) // start fixture IDs high to avoid collisions
}

func nextFixtureID() int64 {
	return fixtureIDCounter.Add(1)
}

var fixtureTime = time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

// CompanyFixture returns a realistic Company entity with all required fields set.
func CompanyFixture(overrides ...func(*entities.Company)) entities.Company {
	c := entities.Company{
		ID:            autotask.Set(nextFixtureID()),
		CompanyName:   autotask.Set("Acme Corporation"),
		CompanyNumber: autotask.Set("ACME-001"),
		Phone:         autotask.Set("555-0100"),
		WebAddress:    autotask.Set("https://acme.example.com"),
		Address1:      autotask.Set("123 Main Street"),
		City:          autotask.Set("Springfield"),
		State:         autotask.Set("IL"),
		PostalCode:    autotask.Set("62701"),
		Country:       autotask.Set("United States"),
		CompanyType:   autotask.Set(1),
		IsActive:      autotask.Set(true),
		CreateDate:    autotask.Set(fixtureTime),
		UserDefinedFields: []autotask.UDF{
			{Name: "CustomerRanking", Value: "Gold"},
		},
	}
	for _, fn := range overrides {
		fn(&c)
	}
	return c
}

// ContactFixture returns a realistic Contact entity.
func ContactFixture(overrides ...func(*entities.Contact)) entities.Contact {
	c := entities.Contact{
		ID:           autotask.Set(nextFixtureID()),
		FirstName:    autotask.Set("Jane"),
		LastName:     autotask.Set("Doe"),
		EmailAddress: autotask.Set("jane.doe@acme.example.com"),
		Phone:        autotask.Set("555-0101"),
		MobilePhone:  autotask.Set("555-0102"),
		CompanyID:    autotask.Set(int64(1001)),
		IsActive:     autotask.Set(true),
		CreateDate:   autotask.Set(fixtureTime),
		UserDefinedFields: []autotask.UDF{
			{Name: "PreferredContact", Value: "Email"},
		},
	}
	for _, fn := range overrides {
		fn(&c)
	}
	return c
}

// TicketFixture returns a realistic Ticket entity.
func TicketFixture(overrides ...func(*entities.Ticket)) entities.Ticket {
	t := entities.Ticket{
		ID:           autotask.Set(nextFixtureID()),
		Title:        autotask.Set("Server intermittently unreachable"),
		Description:  autotask.Set("Users report periodic connectivity issues with the production server."),
		TicketNumber: autotask.Set("T20240115.0001"),
		Status:       autotask.Set(1), // New
		Priority:     autotask.Set(2), // High
		CompanyID:    autotask.Set(int64(1001)),
		ContactID:    autotask.Set(int64(2001)),
		DueDateTime:  autotask.Set(fixtureTime.Add(72 * time.Hour)),
		CreateDate:   autotask.Set(fixtureTime),
		TicketType:   autotask.Set(1), // Service Request
		Source:       autotask.Set(1), // Phone
		UserDefinedFields: []autotask.UDF{
			{Name: "Severity", Value: "P2"},
		},
	}
	for _, fn := range overrides {
		fn(&t)
	}
	return t
}

// TicketNoteFixture returns a realistic TicketNote entity.
func TicketNoteFixture(overrides ...func(*entities.TicketNote)) entities.TicketNote {
	n := entities.TicketNote{
		ID:                autotask.Set(nextFixtureID()),
		TicketID:          autotask.Set(int64(3001)),
		Title:             autotask.Set("Initial investigation"),
		Description:       autotask.Set("Checked server logs, found intermittent OOM events."),
		NoteType:          autotask.Set(1), // Internal
		Publish:           autotask.Set(1),
		CreateDateTime:    autotask.Set(fixtureTime),
		CreatorResourceID: autotask.Set(int64(5001)),
		UserDefinedFields: []autotask.UDF{
			{Name: "NoteCategory", Value: "Investigation"},
		},
	}
	for _, fn := range overrides {
		fn(&n)
	}
	return n
}

// ProjectFixture returns a realistic Project entity.
func ProjectFixture(overrides ...func(*entities.Project)) entities.Project {
	p := entities.Project{
		ID:             autotask.Set(nextFixtureID()),
		ProjectName:    autotask.Set("Infrastructure Upgrade Q1"),
		Description:    autotask.Set("Upgrade all production servers to latest OS."),
		Status:         autotask.Set(1), // Active
		Type:           autotask.Set(1),
		CompanyID:      autotask.Set(int64(1001)),
		StartDateTime:  autotask.Set(fixtureTime),
		EndDateTime:    autotask.Set(fixtureTime.Add(90 * 24 * time.Hour)),
		EstimatedHours: autotask.Set(120.0),
		CreateDateTime: autotask.Set(fixtureTime),
		UserDefinedFields: []autotask.UDF{
			{Name: "Department", Value: "IT"},
		},
	}
	for _, fn := range overrides {
		fn(&p)
	}
	return p
}

// TaskFixture returns a realistic Task entity.
func TaskFixture(overrides ...func(*entities.Task)) entities.Task {
	t := entities.Task{
		ID:                 autotask.Set(nextFixtureID()),
		Title:              autotask.Set("Update server firmware"),
		Description:        autotask.Set("Apply latest firmware patches to production servers."),
		Status:             autotask.Set(1), // New
		Priority:           autotask.Set(2), // Normal
		ProjectID:          autotask.Set(int64(4001)),
		AssignedResourceID: autotask.Set(int64(5001)),
		EstimatedHours:     autotask.Set(4.0),
		StartDateTime:      autotask.Set(fixtureTime),
		EndDateTime:        autotask.Set(fixtureTime.Add(8 * time.Hour)),
		CreateDateTime:     autotask.Set(fixtureTime),
		UserDefinedFields: []autotask.UDF{
			{Name: "TaskCategory", Value: "Maintenance"},
		},
	}
	for _, fn := range overrides {
		fn(&t)
	}
	return t
}

// ResourceFixture returns a realistic Resource entity.
func ResourceFixture(overrides ...func(*entities.Resource)) entities.Resource {
	r := entities.Resource{
		ID:           autotask.Set(nextFixtureID()),
		FirstName:    autotask.Set("John"),
		LastName:     autotask.Set("Smith"),
		Email:        autotask.Set("john.smith@company.example.com"),
		UserName:     autotask.Set("jsmith"),
		Title:        autotask.Set("Senior Systems Engineer"),
		IsActive:     autotask.Set(true),
		ResourceType: autotask.Set(1), // Employee
		LocationID:   autotask.Set(int64(1)),
		UserDefinedFields: []autotask.UDF{
			{Name: "Team", Value: "Infrastructure"},
		},
	}
	for _, fn := range overrides {
		fn(&r)
	}
	return r
}

// ContractFixture returns a realistic Contract entity.
func ContractFixture(overrides ...func(*entities.Contract)) entities.Contract {
	c := entities.Contract{
		ID:             autotask.Set(nextFixtureID()),
		ContractName:   autotask.Set("Annual Support Agreement"),
		Description:    autotask.Set("Covers 24/7 phone and email support."),
		CompanyID:      autotask.Set(int64(1001)),
		ContactID:      autotask.Set(int64(2001)),
		Status:         autotask.Set(1), // Active
		ContractType:   autotask.Set(1), // Time & Materials
		StartDate:      autotask.Set(fixtureTime),
		EndDate:        autotask.Set(fixtureTime.Add(365 * 24 * time.Hour)),
		EstimatedHours: autotask.Set(200.0),
		UserDefinedFields: []autotask.UDF{
			{Name: "SLA", Value: "Premium"},
		},
	}
	for _, fn := range overrides {
		fn(&c)
	}
	return c
}

// ConfigurationItemFixture returns a realistic ConfigurationItem entity.
func ConfigurationItemFixture(overrides ...func(*entities.ConfigurationItem)) entities.ConfigurationItem {
	ci := entities.ConfigurationItem{
		ID:                    autotask.Set(nextFixtureID()),
		ReferenceTitle:        autotask.Set("PROD-WEB-01"),
		ReferenceNumber:       autotask.Set("CI-2024-001"),
		CompanyID:             autotask.Set(int64(1001)),
		ConfigurationItemType: autotask.Set(1), // Server
		IsActive:              autotask.Set(true),
		InstallDate:           autotask.Set(fixtureTime),
		SerialNumber:          autotask.Set("SN-ABCD-1234"),
		Location:              autotask.Set("Data Center A, Rack 12"),
		CreateDate:            autotask.Set(fixtureTime),
		UserDefinedFields: []autotask.UDF{
			{Name: "OS", Value: "Ubuntu 22.04 LTS"},
		},
	}
	for _, fn := range overrides {
		fn(&ci)
	}
	return ci
}

// TimeEntryFixture returns a realistic TimeEntry entity.
func TimeEntryFixture(overrides ...func(*entities.TimeEntry)) entities.TimeEntry {
	te := entities.TimeEntry{
		ID:            autotask.Set(nextFixtureID()),
		TicketID:      autotask.Set(int64(3001)),
		ResourceID:    autotask.Set(int64(5001)),
		DateWorked:    autotask.Set(fixtureTime),
		StartDateTime: autotask.Set(fixtureTime),
		EndDateTime:   autotask.Set(fixtureTime.Add(2 * time.Hour)),
		HoursWorked:   autotask.Set(2.0),
		SummaryNotes:  autotask.Set("Investigated server connectivity issue."),
		BillingCodeID: autotask.Set(int64(100)),
		UserDefinedFields: []autotask.UDF{
			{Name: "BillableType", Value: "Chargeable"},
		},
	}
	for _, fn := range overrides {
		fn(&te)
	}
	return te
}
