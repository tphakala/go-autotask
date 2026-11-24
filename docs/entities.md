# Entities, metadata, and code generation

Back to [README](../README.md).

## Available entities

The `entities` package ships type-safe structs, some generated with `autotask-gen` and some hand-written, for:

`BillingCode`, `BillingItem`, `BillingItemApprovalLevel`, `ChangeRequestLink`, `ClientPortalUser`, `Company`, `CompanyNote`, `ConfigurationItem`, `ConfigurationItemAttachment`, `ConfigurationItemBillingProductAssociation`, `ConfigurationItemCategory`, `ConfigurationItemCategoryUdfAssociation`, `ConfigurationItemDnsRecord`, `ConfigurationItemNote`, `ConfigurationItemNoteAttachment`, `ConfigurationItemRelatedItem`, `ConfigurationItemSslSubjectAlternativeName`, `ConfigurationItemType`, `Contact`, `ContactGroup`, `ContactGroupContact`, `Contract`, `Department`, `ExpenseItem`, `ExpenseReport`, `Invoice`, `Opportunity`, `Product`, `Project`, `ProjectNote`, `Quote`, `QuoteItem`, `Resource`, `ResourceAttachment`, `ResourceRole`, `ResourceRoleDepartment`, `ResourceRoleQueue`, `ResourceServiceDeskRole`, `ResourceSkill`, `Role`, `Service`, `ServiceBundle`, `Skill`, `Task`, `Ticket`, `TicketAdditionalConfigurationItem`, `TicketAttachment`, `TicketNote`, `TicketNoteAttachment`, `TimeEntry`

All entities use `Optional[T]` fields, and all except `ChangeRequestLink`, `TicketNoteAttachment`, `TicketAdditionalConfigurationItem`, the `ConfigurationItem*` types other than `ConfigurationItem` itself, and the user management types other than `Contact` and `Resource` (`ClientPortalUser`, `ContactGroup`, `ContactGroupContact`, `Role`, `Skill`, and the `Resource*` types other than `Resource`; Autotask gives them none) support user-defined fields via `UserDefinedFields []autotask.UDF`. `Department` and `Resource` carry the field although the API lists no user-defined fields for them.

For an entity not listed here, use the [raw operations](operations.md#raw-operations) (`map[string]any`) API or generate a struct with [`autotask-gen`](#code-generation).

## Configuration items (CMDB)

`ConfigurationItem` is a root entity: `Create` it with `ProductID`, `CompanyID`, and `IsActive` set, and change it with `Update`. Autotask has no delete for configuration items; set `IsActive` to false to retire one. Its notes, attachments, related items, billing product associations, DNS records, and SSL subject alternative names are child entities, reached through the parent with the [child calls](operations.md#child-entities).

```go
ci, err := autotask.Create(ctx, client, &entities.ConfigurationItem{
	ProductID:      autotask.Set(productID),
	CompanyID:      autotask.Set(companyID),
	IsActive:       autotask.Set(true),
	ReferenceTitle: autotask.Set("PROD-WEB-01"),
})
ciID, _ := ci.ID.Get()

// Add a note to it, then edit the note: PATCH /v1.0/ConfigurationItems/{ciID}/Notes
note, err := autotask.CreateChild[entities.ConfigurationItem](ctx, client, ciID, &entities.ConfigurationItemNote{
	Title:       autotask.Set("Rack move"),
	Description: autotask.Set("Moved to rack 12."),
	NoteType:    autotask.Set(noteType), // a value of the NoteType picklist
})
note.Title = autotask.Set("Rack move (done)")
note, err = autotask.UpdateChild[entities.ConfigurationItem](ctx, client, ciID, note)

// Relate it to another configuration item
_, err = autotask.CreateChild[entities.ConfigurationItem](ctx, client, ciID, &entities.ConfigurationItemRelatedItem{
	RelatedConfigurationItemID: autotask.Set(otherCIID),
	Relationship:               autotask.Set(relationship), // a value of the Relationship picklist
})

// Delete an attachment: DELETE /v1.0/ConfigurationItems/{ciID}/Attachments/{id}
err = autotask.DeleteChild[entities.ConfigurationItem, entities.ConfigurationItemAttachment](ctx, client, ciID, attachmentID)
```

The other types follow the same rule: their doc comments name the typed calls that reach the operations the REST swagger lists for them. `ConfigurationItemType` and `ConfigurationItemCategory` are root entities. `ConfigurationItemDnsRecord` and `ConfigurationItemSslSubjectAlternativeName` can be listed and deleted but not created or updated. `TicketAdditionalConfigurationItem` is a child of `Ticket`.

## Users and contacts

Internal users are `Resource` entities. The API supports `Get`, `List`, `Count`, and `Update` for them, with no create or delete, so users are added in Autotask; update a resource with `IsActive` set to false to deactivate it (not checked against a live tenant). Their assignments are child entities of the resource, reached with the child calls: `ResourceRoleDepartment`, `ResourceRoleQueue`, and `ResourceServiceDeskRole` can be listed, created, and updated, `ResourceSkill` listed and updated, `ResourceAttachment` listed, created, and deleted, and `ResourceRole` only listed.

Customer users are `Contact` entities. The REST swagger offers no root write for them: create, update, and delete a contact through its company with `CreateChild[entities.Company]`, `UpdateChild[entities.Company]`, and `DeleteChild[entities.Company, entities.Contact]`. `ClientPortalUser` is a root entity with create and update, and no delete. `ContactGroup` is a root entity with create, update, and delete, and its members are `ContactGroupContact` items, created and deleted through the group.

```go
// List the active resources
active, err := autotask.List[entities.Resource](ctx, client,
	autotask.NewQuery().Where("isActive", autotask.OpEq, true))

// Deactivate one: send only ID and the field to change, PATCH /v1.0/Resources
_, err = autotask.Update(ctx, client, &entities.Resource{
	ID:       autotask.Set(resourceID),
	IsActive: autotask.Set(false),
})

// Assign it to a department with a role: POST /v1.0/Resources/{resourceID}/RoleDepartments
_, err = autotask.CreateChild[entities.Resource](ctx, client, resourceID, &entities.ResourceRoleDepartment{
	ResourceID:       autotask.Set(resourceID), // read-only per the metadata; whether the URL alone suffices is not checked against a live tenant
	DepartmentID:     autotask.Set(departmentID),
	RoleID:           autotask.Set(roleID),
	IsActive:         autotask.Set(true),
	IsDefault:        autotask.Set(true),
	IsDepartmentLead: autotask.Set(false),
})

// Create a contact: POST /v1.0/Companies/{companyID}/Contacts
contact, err := autotask.CreateChild[entities.Company](ctx, client, companyID, &entities.Contact{
	CompanyID: autotask.Set(companyID),
	FirstName: autotask.Set("Jane"),
	LastName:  autotask.Set("Doe"),
	IsActive:  autotask.Set(int64(1)),
})
if err != nil {
	return err
}

// Edit the contact: PATCH /v1.0/Companies/{companyID}/Contacts with only the ID and the changed field
contactID, _ := contact.ID.Get()
_, err = autotask.UpdateChild[entities.Company](ctx, client, companyID, &entities.Contact{
	ID:    autotask.Set(contactID),
	Phone: autotask.Set("555-0101"),
})

// Add the contact to a contact group: POST /v1.0/ContactGroups/{groupID}/Contacts
_, err = autotask.CreateChild[entities.ContactGroup](ctx, client, groupID, &entities.ContactGroupContact{
	ContactGroupID: autotask.Set(groupID),
	ContactID:      autotask.Set(contactID),
})
```

The other types' doc comments name the typed calls that reach the operations the REST swagger lists for them. `Role` and `Department` have root create and update but no delete, and `Skill` and `ResourceRole` are read-only. Write behavior beyond what the swagger and the field metadata state has not been checked against a live tenant. Picklist fields (for example a resource's `EmailTypeCode`, `PayrollType`, and `UserType`) take values from `metadata.GetPickList`.

## Problem, incident, and change request tickets

An incident ticket points at its problem ticket through `Ticket.ProblemTicketID`. The JSON field is `problemTicketId`, with a lowercase `d`, and Autotask accepts it only on Incident tickets. List the incidents of a problem by filtering on it; the filter needs the field to be queryable in your instance, which `metadata.GetFields` reports as `IsQueryable`. Ticket type values come from `metadata.GetPickList(ctx, client, "Tickets", "ticketType")`.

```go
incidents, err := autotask.List[entities.Ticket](ctx, client,
	autotask.NewQuery().Where("problemTicketId", autotask.OpEq, problemID),
)
```

`ChangeRequestLink` links a Change Request ticket to a Problem or Incident ticket. Autotask has no update for this entity, so delete a link and create a new one instead of calling `Update`. `Create` sets the new link's `ID` on the returned entity.

```go
_, err := autotask.Create(ctx, client, &entities.ChangeRequestLink{
	ChangeRequestTicketID:     autotask.Set(changeRequestID),
	ProblemOrIncidentTicketID: autotask.Set(incidentID),
})

links, err := autotask.List[entities.ChangeRequestLink](ctx, client,
	autotask.NewQuery().
		Where("changeRequestTicketID", autotask.OpEq, changeRequestID).
		Where("problemOrIncidentTicketID", autotask.OpEq, incidentID),
)
if len(links) > 0 {
	id, _ := links[0].ID.Get()
	err = autotask.Delete[entities.ChangeRequestLink](ctx, client, id)
}
```

The `autotasktest` server accepts PATCH for every entity unless the test adds `autotasktest.WithoutUpdateSupport("ChangeRequestLinks")`, which answers it with a 400 (see [Testing](testing.md#simulating-api-write-restrictions)).

## Metadata

Query entity structure at runtime:

```go
import "github.com/tphakala/go-autotask/metadata"

fields, err := metadata.GetFields(ctx, client, "Tickets")
for _, f := range fields {
	fmt.Printf("%s (%s) required=%v\n", f.Name, f.Type, f.IsRequired)
}

// Picklist values for a specific field
values, err := metadata.GetPickList(ctx, client, "Tickets", "status")
for _, v := range values {
	fmt.Printf("%s = %s\n", v.Value, v.Label)
}

udfs, err := metadata.GetUDFs(ctx, client, "Tickets")

info, err := metadata.GetEntityInfo(ctx, client, "Tickets")
fmt.Printf("canCreate=%v canQuery=%v\n", info.CanCreate, info.CanQuery)
```

## Code generation

Generate entity structs from live API metadata:

```sh
go run ./cmd/autotask-gen \
	-username user@example.com \
	-secret s3cret \
	-integration-code INT123 \
	-output ./entities
```

Run from a checkout of this repository, this regenerates the generated files in `./entities`; files generated earlier are overwritten (each file is written to a temporary file and renamed into place, so a run that fails to format an entity leaves its earlier file as it was, and an interrupted run leaves each file either as it was or fully replaced), except one whose type is declared in another file: it is skipped and stays in place, so delete it when it is a leftover. An entity whose Go type is already declared in another Go source file of the output directory (test files, names starting with `.` or `_`, and entries that are not regular files or symbolic links to one are not read), such as the hand-written `Ticket` and `Company`, is skipped with a warning. A file that does not start with the `// Code generated by autotask-gen. DO NOT EDIT.` header is never overwritten. The command exits with an error when no entity was generated. Use `-entities Name1,Name2` to pick entities by their Autotask (plural) name and `-output` to pick the directory; generated files declare `package entities`.
