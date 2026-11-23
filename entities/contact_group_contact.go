package entities

import (
	autotask "github.com/tphakala/go-autotask"
)

// ContactGroupContact represents an Autotask ContactGroupContact entity, the
// membership of a contact in a contact group.
//
// Get, List and Count work at the root. Per the REST swagger, the writes go
// through the group: autotask.ListChild[ContactGroup, ContactGroupContact]
// lists /v1.0/ContactGroups/{groupID}/Contacts,
// autotask.CreateChild[ContactGroup] posts to that path, and
// autotask.DeleteChild[ContactGroup, ContactGroupContact] sends DELETE to that
// path plus /{id}. The API has no update for this entity. autotask.Create and
// autotask.Delete send root URLs that the REST swagger does not list for it, so
// use the child calls. The JSON names ContactGroupID and ContactID are
// contactGroupId and contactId, with a lowercase d. It has no user-defined
// fields.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not regenerate
// it.
type ContactGroupContact struct {
	ContactGroupID autotask.Optional[int64] `json:"contactGroupId,omitzero"`
	ContactID      autotask.Optional[int64] `json:"contactId,omitzero"`
	ID             autotask.Optional[int64] `json:"id,omitzero"`
}

// EntityName returns the Autotask API entity name for contact group contacts.
func (ContactGroupContact) EntityName() string { return "ContactGroupContacts" }
