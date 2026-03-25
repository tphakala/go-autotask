package autotask_test

import (
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/entities"
)

// ---------- Company ----------

func TestCompanyCRUD(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(company))

	t.Run("Get", func(t *testing.T) {
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
	})

	t.Run("List", func(t *testing.T) {
		q := autotask.NewQuery().Where("companyName", autotask.OpEq, "Acme Corporation")
		items, err := autotask.List[entities.Company](t.Context(), client, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 {
			t.Fatal("expected at least 1 item")
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := autotask.Count[entities.Company](t.Context(), client, autotask.NewQuery())
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
	})

	t.Run("Create", func(t *testing.T) {
		newCo := autotasktest.CompanyFixture(func(c *entities.Company) {
			c.ID = autotask.Optional[int64]{}
			c.CompanyName = autotask.Set("New Corp")
		})
		result, err := autotask.Create(t.Context(), client, &newCo)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("Update", func(t *testing.T) {
		updated := company
		updated.Phone = autotask.Set("555-9999")
		result, err := autotask.Update(t.Context(), client, &updated)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("CreateWithUDF", func(t *testing.T) {
		co := autotasktest.CompanyFixture(func(c *entities.Company) {
			c.ID = autotask.Optional[int64]{}
			c.UserDefinedFields = []autotask.UDF{{Name: "CustomerRanking", Value: "Platinum"}}
		})
		result, err := autotask.Create(t.Context(), client, &co)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})
}

// ---------- Contact ----------

func TestContactCRUD(t *testing.T) {
	t.Parallel()
	contact := autotasktest.ContactFixture()
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(contact),
		autotasktest.WithDeleteSupport("Contacts"),
	)

	t.Run("Get", func(t *testing.T) {
		id, _ := contact.ID.Get()
		got, err := autotask.Get[entities.Contact](t.Context(), client, id)
		if err != nil {
			t.Fatal(err)
		}
		gotLast, _ := got.LastName.Get()
		wantLast, _ := contact.LastName.Get()
		if gotLast != wantLast {
			t.Fatalf("LastName = %q, want %q", gotLast, wantLast)
		}
	})

	t.Run("List", func(t *testing.T) {
		q := autotask.NewQuery().Where("lastName", autotask.OpEq, "Doe")
		items, err := autotask.List[entities.Contact](t.Context(), client, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 {
			t.Fatal("expected at least 1 item")
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := autotask.Count[entities.Contact](t.Context(), client, autotask.NewQuery())
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
	})

	t.Run("Create", func(t *testing.T) {
		newContact := autotasktest.ContactFixture(func(c *entities.Contact) {
			c.ID = autotask.Optional[int64]{}
			c.FirstName = autotask.Set("Bob")
			c.LastName = autotask.Set("Jones")
		})
		result, err := autotask.Create(t.Context(), client, &newContact)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("Update", func(t *testing.T) {
		updated := contact
		updated.Phone = autotask.Set("555-8888")
		result, err := autotask.Update(t.Context(), client, &updated)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		id, _ := contact.ID.Get()
		err := autotask.Delete[entities.Contact](t.Context(), client, id)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("CreateWithUDF", func(t *testing.T) {
		co := autotasktest.ContactFixture(func(c *entities.Contact) {
			c.ID = autotask.Optional[int64]{}
			c.UserDefinedFields = []autotask.UDF{{Name: "PreferredContact", Value: "Phone"}}
		})
		result, err := autotask.Create(t.Context(), client, &co)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})
}

// ---------- Ticket ----------

func TestTicketCRUD(t *testing.T) {
	t.Parallel()
	ticket := autotasktest.TicketFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(ticket))

	t.Run("Get", func(t *testing.T) {
		id, _ := ticket.ID.Get()
		got, err := autotask.Get[entities.Ticket](t.Context(), client, id)
		if err != nil {
			t.Fatal(err)
		}
		gotTitle, _ := got.Title.Get()
		wantTitle, _ := ticket.Title.Get()
		if gotTitle != wantTitle {
			t.Fatalf("Title = %q, want %q", gotTitle, wantTitle)
		}
	})

	t.Run("List", func(t *testing.T) {
		q := autotask.NewQuery().Where("status", autotask.OpEq, 1)
		items, err := autotask.List[entities.Ticket](t.Context(), client, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 {
			t.Fatal("expected at least 1 item")
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := autotask.Count[entities.Ticket](t.Context(), client, autotask.NewQuery())
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
	})

	t.Run("Create", func(t *testing.T) {
		newTicket := autotasktest.TicketFixture(func(tk *entities.Ticket) {
			tk.ID = autotask.Optional[int64]{}
			tk.Title = autotask.Set("New ticket")
		})
		result, err := autotask.Create(t.Context(), client, &newTicket)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("Update", func(t *testing.T) {
		updated := ticket
		updated.Priority = autotask.Set(int64(3))
		result, err := autotask.Update(t.Context(), client, &updated)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("CreateWithUDF", func(t *testing.T) {
		tk := autotasktest.TicketFixture(func(tk *entities.Ticket) {
			tk.ID = autotask.Optional[int64]{}
			tk.UserDefinedFields = []autotask.UDF{{Name: "Severity", Value: "P1"}}
		})
		result, err := autotask.Create(t.Context(), client, &tk)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})
}

// ---------- TicketNote ----------

func TestTicketNoteCRUD(t *testing.T) {
	t.Parallel()
	note := autotasktest.TicketNoteFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(note))

	t.Run("Get", func(t *testing.T) {
		id, _ := note.ID.Get()
		got, err := autotask.Get[entities.TicketNote](t.Context(), client, id)
		if err != nil {
			t.Fatal(err)
		}
		gotTitle, _ := got.Title.Get()
		wantTitle, _ := note.Title.Get()
		if gotTitle != wantTitle {
			t.Fatalf("Title = %q, want %q", gotTitle, wantTitle)
		}
	})

	t.Run("List", func(t *testing.T) {
		q := autotask.NewQuery().Where("noteType", autotask.OpEq, 1)
		items, err := autotask.List[entities.TicketNote](t.Context(), client, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 {
			t.Fatal("expected at least 1 item")
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := autotask.Count[entities.TicketNote](t.Context(), client, autotask.NewQuery())
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
	})

	t.Run("Create", func(t *testing.T) {
		newNote := autotasktest.TicketNoteFixture(func(n *entities.TicketNote) {
			n.ID = autotask.Optional[int64]{}
			n.Title = autotask.Set("Follow-up note")
		})
		result, err := autotask.Create(t.Context(), client, &newNote)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("Update", func(t *testing.T) {
		updated := note
		updated.Description = autotask.Set("Updated description")
		result, err := autotask.Update(t.Context(), client, &updated)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("CreateWithUDF", func(t *testing.T) {
		n := autotasktest.TicketNoteFixture(func(n *entities.TicketNote) {
			n.ID = autotask.Optional[int64]{}
			n.UserDefinedFields = []autotask.UDF{{Name: "NoteCategory", Value: "Resolution"}}
		})
		result, err := autotask.Create(t.Context(), client, &n)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})
}

// ---------- Project ----------

func TestProjectCRUD(t *testing.T) {
	t.Parallel()
	project := autotasktest.ProjectFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(project))

	t.Run("Get", func(t *testing.T) {
		id, _ := project.ID.Get()
		got, err := autotask.Get[entities.Project](t.Context(), client, id)
		if err != nil {
			t.Fatal(err)
		}
		gotName, _ := got.ProjectName.Get()
		wantName, _ := project.ProjectName.Get()
		if gotName != wantName {
			t.Fatalf("ProjectName = %q, want %q", gotName, wantName)
		}
	})

	t.Run("List", func(t *testing.T) {
		q := autotask.NewQuery().Where("status", autotask.OpEq, 1)
		items, err := autotask.List[entities.Project](t.Context(), client, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 {
			t.Fatal("expected at least 1 item")
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := autotask.Count[entities.Project](t.Context(), client, autotask.NewQuery())
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
	})

	t.Run("Create", func(t *testing.T) {
		newProject := autotasktest.ProjectFixture(func(p *entities.Project) {
			p.ID = autotask.Optional[int64]{}
			p.ProjectName = autotask.Set("New Project")
		})
		result, err := autotask.Create(t.Context(), client, &newProject)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("Update", func(t *testing.T) {
		updated := project
		updated.Description = autotask.Set("Updated project description")
		result, err := autotask.Update(t.Context(), client, &updated)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("CreateWithUDF", func(t *testing.T) {
		p := autotasktest.ProjectFixture(func(p *entities.Project) {
			p.ID = autotask.Optional[int64]{}
			p.UserDefinedFields = []autotask.UDF{{Name: "Department", Value: "Engineering"}}
		})
		result, err := autotask.Create(t.Context(), client, &p)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})
}

// ---------- Task ----------

func TestTaskCRUD(t *testing.T) {
	t.Parallel()
	task := autotasktest.TaskFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(task))

	t.Run("Get", func(t *testing.T) {
		id, _ := task.ID.Get()
		got, err := autotask.Get[entities.Task](t.Context(), client, id)
		if err != nil {
			t.Fatal(err)
		}
		gotTitle, _ := got.Title.Get()
		wantTitle, _ := task.Title.Get()
		if gotTitle != wantTitle {
			t.Fatalf("Title = %q, want %q", gotTitle, wantTitle)
		}
	})

	t.Run("List", func(t *testing.T) {
		q := autotask.NewQuery().Where("status", autotask.OpEq, 1)
		items, err := autotask.List[entities.Task](t.Context(), client, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 {
			t.Fatal("expected at least 1 item")
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := autotask.Count[entities.Task](t.Context(), client, autotask.NewQuery())
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
	})

	t.Run("Create", func(t *testing.T) {
		newTask := autotasktest.TaskFixture(func(tk *entities.Task) {
			tk.ID = autotask.Optional[int64]{}
			tk.Title = autotask.Set("New task")
		})
		result, err := autotask.Create(t.Context(), client, &newTask)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("Update", func(t *testing.T) {
		updated := task
		updated.Description = autotask.Set("Updated task description")
		result, err := autotask.Update(t.Context(), client, &updated)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("CreateWithUDF", func(t *testing.T) {
		tk := autotasktest.TaskFixture(func(tk *entities.Task) {
			tk.ID = autotask.Optional[int64]{}
			tk.UserDefinedFields = []autotask.UDF{{Name: "TaskCategory", Value: "Upgrade"}}
		})
		result, err := autotask.Create(t.Context(), client, &tk)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})
}

// ---------- Resource ----------

func TestResourceCRUD(t *testing.T) {
	t.Parallel()
	resource := autotasktest.ResourceFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(resource))

	t.Run("Get", func(t *testing.T) {
		id, _ := resource.ID.Get()
		got, err := autotask.Get[entities.Resource](t.Context(), client, id)
		if err != nil {
			t.Fatal(err)
		}
		gotFirst, _ := got.FirstName.Get()
		wantFirst, _ := resource.FirstName.Get()
		if gotFirst != wantFirst {
			t.Fatalf("FirstName = %q, want %q", gotFirst, wantFirst)
		}
	})

	t.Run("List", func(t *testing.T) {
		q := autotask.NewQuery().Where("lastName", autotask.OpEq, "Smith")
		items, err := autotask.List[entities.Resource](t.Context(), client, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 {
			t.Fatal("expected at least 1 item")
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := autotask.Count[entities.Resource](t.Context(), client, autotask.NewQuery())
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
	})

	t.Run("Create", func(t *testing.T) {
		newResource := autotasktest.ResourceFixture(func(r *entities.Resource) {
			r.ID = autotask.Optional[int64]{}
			r.FirstName = autotask.Set("Alice")
			r.LastName = autotask.Set("Brown")
		})
		result, err := autotask.Create(t.Context(), client, &newResource)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("Update", func(t *testing.T) {
		updated := resource
		updated.Title = autotask.Set("Lead Engineer")
		result, err := autotask.Update(t.Context(), client, &updated)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("CreateWithUDF", func(t *testing.T) {
		r := autotasktest.ResourceFixture(func(r *entities.Resource) {
			r.ID = autotask.Optional[int64]{}
			r.UserDefinedFields = []autotask.UDF{{Name: "Team", Value: "Platform"}}
		})
		result, err := autotask.Create(t.Context(), client, &r)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})
}

// ---------- Contract ----------

func TestContractCRUD(t *testing.T) {
	t.Parallel()
	contract := autotasktest.ContractFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(contract))

	t.Run("Get", func(t *testing.T) {
		id, _ := contract.ID.Get()
		got, err := autotask.Get[entities.Contract](t.Context(), client, id)
		if err != nil {
			t.Fatal(err)
		}
		gotName, _ := got.ContractName.Get()
		wantName, _ := contract.ContractName.Get()
		if gotName != wantName {
			t.Fatalf("ContractName = %q, want %q", gotName, wantName)
		}
	})

	t.Run("List", func(t *testing.T) {
		q := autotask.NewQuery().Where("contractName", autotask.OpEq, "Annual Support Agreement")
		items, err := autotask.List[entities.Contract](t.Context(), client, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 {
			t.Fatal("expected at least 1 item")
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := autotask.Count[entities.Contract](t.Context(), client, autotask.NewQuery())
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
	})

	t.Run("Create", func(t *testing.T) {
		newContract := autotasktest.ContractFixture(func(c *entities.Contract) {
			c.ID = autotask.Optional[int64]{}
			c.ContractName = autotask.Set("New Contract")
		})
		result, err := autotask.Create(t.Context(), client, &newContract)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("Update", func(t *testing.T) {
		updated := contract
		updated.Description = autotask.Set("Updated contract description")
		result, err := autotask.Update(t.Context(), client, &updated)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("CreateWithUDF", func(t *testing.T) {
		c := autotasktest.ContractFixture(func(c *entities.Contract) {
			c.ID = autotask.Optional[int64]{}
			c.UserDefinedFields = []autotask.UDF{{Name: "SLA", Value: "Enterprise"}}
		})
		result, err := autotask.Create(t.Context(), client, &c)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})
}

// ---------- ConfigurationItem ----------

func TestConfigurationItemCRUD(t *testing.T) {
	t.Parallel()
	ci := autotasktest.ConfigurationItemFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(ci))

	t.Run("Get", func(t *testing.T) {
		id, _ := ci.ID.Get()
		got, err := autotask.Get[entities.ConfigurationItem](t.Context(), client, id)
		if err != nil {
			t.Fatal(err)
		}
		gotTitle, _ := got.ReferenceTitle.Get()
		wantTitle, _ := ci.ReferenceTitle.Get()
		if gotTitle != wantTitle {
			t.Fatalf("ReferenceTitle = %q, want %q", gotTitle, wantTitle)
		}
	})

	t.Run("List", func(t *testing.T) {
		q := autotask.NewQuery().Where("referenceTitle", autotask.OpEq, "PROD-WEB-01")
		items, err := autotask.List[entities.ConfigurationItem](t.Context(), client, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 {
			t.Fatal("expected at least 1 item")
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := autotask.Count[entities.ConfigurationItem](t.Context(), client, autotask.NewQuery())
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
	})

	t.Run("Create", func(t *testing.T) {
		newCI := autotasktest.ConfigurationItemFixture(func(c *entities.ConfigurationItem) {
			c.ID = autotask.Optional[int64]{}
			c.ReferenceTitle = autotask.Set("PROD-WEB-02")
		})
		result, err := autotask.Create(t.Context(), client, &newCI)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("Update", func(t *testing.T) {
		updated := ci
		updated.Location = autotask.Set("Data Center B, Rack 5")
		result, err := autotask.Update(t.Context(), client, &updated)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("CreateWithUDF", func(t *testing.T) {
		c := autotasktest.ConfigurationItemFixture(func(c *entities.ConfigurationItem) {
			c.ID = autotask.Optional[int64]{}
			c.UserDefinedFields = []autotask.UDF{{Name: "OS", Value: "Rocky Linux 9"}}
		})
		result, err := autotask.Create(t.Context(), client, &c)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})
}

// ---------- TimeEntry ----------

func TestTimeEntryCRUD(t *testing.T) {
	t.Parallel()
	te := autotasktest.TimeEntryFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(te))

	t.Run("Get", func(t *testing.T) {
		id, _ := te.ID.Get()
		got, err := autotask.Get[entities.TimeEntry](t.Context(), client, id)
		if err != nil {
			t.Fatal(err)
		}
		gotNotes, _ := got.SummaryNotes.Get()
		wantNotes, _ := te.SummaryNotes.Get()
		if gotNotes != wantNotes {
			t.Fatalf("SummaryNotes = %q, want %q", gotNotes, wantNotes)
		}
	})

	t.Run("List", func(t *testing.T) {
		q := autotask.NewQuery().Where("hoursWorked", autotask.OpGte, 1.0)
		items, err := autotask.List[entities.TimeEntry](t.Context(), client, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 {
			t.Fatal("expected at least 1 item")
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := autotask.Count[entities.TimeEntry](t.Context(), client, autotask.NewQuery())
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
	})

	t.Run("Create", func(t *testing.T) {
		newTE := autotasktest.TimeEntryFixture(func(te *entities.TimeEntry) {
			te.ID = autotask.Optional[int64]{}
			te.SummaryNotes = autotask.Set("Additional investigation time")
		})
		result, err := autotask.Create(t.Context(), client, &newTE)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("Update", func(t *testing.T) {
		updated := te
		updated.SummaryNotes = autotask.Set("Updated summary notes")
		result, err := autotask.Update(t.Context(), client, &updated)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("CreateWithUDF", func(t *testing.T) {
		entry := autotasktest.TimeEntryFixture(func(te *entities.TimeEntry) {
			te.ID = autotask.Optional[int64]{}
			te.UserDefinedFields = []autotask.UDF{{Name: "BillableType", Value: "Non-Chargeable"}}
		})
		result, err := autotask.Create(t.Context(), client, &entry)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})
}
