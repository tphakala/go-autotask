package entities

import (
	"reflect"
	"strings"
	"testing"
	"unicode"

	autotask "github.com/tphakala/go-autotask"
)

// TestCMDBEntityShapes pins the structure shared by the configuration item
// types: the entity name, that user-defined fields exist on ConfigurationItem
// only, and that every other member is an Optional with a JSON name equal to
// the Go name with its first letter lowercased (ID maps to id) and the omitzero
// option.
func TestCMDBEntityShapes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value      autotask.Entity
		wantEntity string
		wantUDF    bool
	}{
		{ConfigurationItem{}, "ConfigurationItems", true},
		{ConfigurationItemType{}, "ConfigurationItemTypes", false},
		{ConfigurationItemCategory{}, "ConfigurationItemCategories", false},
		{ConfigurationItemCategoryUdfAssociation{}, "ConfigurationItemCategoryUdfAssociations", false},
		{ConfigurationItemNote{}, "ConfigurationItemNotes", false},
		{ConfigurationItemAttachment{}, "ConfigurationItemAttachments", false},
		{ConfigurationItemNoteAttachment{}, "ConfigurationItemNoteAttachments", false},
		{ConfigurationItemRelatedItem{}, "ConfigurationItemRelatedItems", false},
		{ConfigurationItemBillingProductAssociation{}, "ConfigurationItemBillingProductAssociations", false},
		{ConfigurationItemDnsRecord{}, "ConfigurationItemDnsRecords", false},
		{ConfigurationItemSslSubjectAlternativeName{}, "ConfigurationItemSslSubjectAlternativeNames", false},
		{TicketAdditionalConfigurationItem{}, "TicketAdditionalConfigurationItems", false},
	}
	for _, tt := range tests {
		typ := reflect.TypeOf(tt.value)
		t.Run(typ.Name(), func(t *testing.T) {
			t.Parallel()
			if got := tt.value.EntityName(); got != tt.wantEntity {
				t.Errorf("EntityName() = %q, want %q", got, tt.wantEntity)
			}
			hasUDF := false
			for field := range typ.Fields() {
				if field.Name == "UserDefinedFields" {
					hasUDF = true
					continue
				}
				if !strings.HasPrefix(field.Type.Name(), "Optional[") || field.Type.PkgPath() != reflect.TypeFor[autotask.UDF]().PkgPath() {
					t.Errorf("%s has type %s, want autotask.Optional[...]", field.Name, field.Type)
				}
				name := field.Name
				if name == "ID" {
					name = "id"
				} else {
					runes := []rune(name)
					runes[0] = unicode.ToLower(runes[0])
					name = string(runes)
				}
				if want := name + ",omitzero"; field.Tag.Get("json") != want {
					t.Errorf("%s has json tag %q, want %q", field.Name, field.Tag.Get("json"), want)
				}
			}
			if hasUDF != tt.wantUDF {
				t.Errorf("has UserDefinedFields = %v, want %v", hasUDF, tt.wantUDF)
			}
		})
	}
}
