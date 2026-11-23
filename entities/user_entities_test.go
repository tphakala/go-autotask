package entities

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	autotask "github.com/tphakala/go-autotask"
)

// wantField is the expected JSON name and Optional element type of one field.
type wantField struct {
	json   string
	goType string
}

// TestUserManagementEntityFields pins the field list of Contact, Resource and
// the user management types: the entity name, the JSON names in metadata
// order, the Optional element type of each, the omitzero option, and that
// user-defined fields exist on Contact and Resource only. The expected rows
// follow the entity field metadata of a live tenant (captured 2026-11-23).
func TestUserManagementEntityFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value      autotask.Entity
		wantEntity string
		wantUDF    bool
		wantFields []wantField
	}{
		{
			value: Contact{}, wantEntity: "Contacts", wantUDF: true,
			wantFields: []wantField{
				{"additionalAddressInformation", "string"},
				{"addressLine", "string"},
				{"addressLine1", "string"},
				{"alternatePhone", "string"},
				{"apiVendorID", "int64"},
				{"billingContact", "bool"},
				{"bulkEmailOptOutTime", "time.Time"},
				{"city", "string"},
				{"companyID", "int64"},
				{"companyLocationID", "int64"},
				{"countryID", "int64"},
				{"createDate", "time.Time"},
				{"emailAddress", "string"},
				{"emailAddress2", "string"},
				{"emailAddress3", "string"},
				{"extension", "string"},
				{"externalID", "string"},
				{"facebookUrl", "string"},
				{"faxNumber", "string"},
				{"firstName", "string"},
				{"id", "int64"},
				{"impersonatorCreatorResourceID", "int64"},
				{"isActive", "int64"},
				{"isOptedOutFromBulkEmail", "bool"},
				{"lastActivityDate", "time.Time"},
				{"lastModifiedDate", "time.Time"},
				{"lastName", "string"},
				{"linkedInUrl", "string"},
				{"middleInitial", "string"},
				{"mobilePhone", "string"},
				{"namePrefix", "int64"},
				{"nameSuffix", "int64"},
				{"note", "string"},
				{"phone", "string"},
				{"primaryContact", "bool"},
				{"receivesEmailNotifications", "bool"},
				{"roomNumber", "string"},
				{"solicitationOptOut", "bool"},
				{"solicitationOptOutTime", "time.Time"},
				{"state", "string"},
				{"surveyOptOut", "bool"},
				{"title", "string"},
				{"twitterUrl", "string"},
				{"zipCode", "string"},
			},
		},
		{
			value: Resource{}, wantEntity: "Resources", wantUDF: true,
			wantFields: []wantField{
				{"accountingReferenceID", "string"},
				{"dateFormat", "string"},
				{"defaultServiceDeskRoleID", "int64"},
				{"email", "string"},
				{"email2", "string"},
				{"email3", "string"},
				{"emailTypeCode", "string"},
				{"emailTypeCode2", "string"},
				{"emailTypeCode3", "string"},
				{"firstName", "string"},
				{"gender", "string"},
				{"greeting", "int64"},
				{"hireDate", "time.Time"},
				{"homePhone", "string"},
				{"id", "int64"},
				{"initials", "string"},
				{"internalCost", "float64"},
				{"isActive", "bool"},
				{"lastName", "string"},
				{"licenseType", "int64"},
				{"locationID", "int64"},
				{"middleName", "string"},
				{"mobilePhone", "string"},
				{"numberFormat", "string"},
				{"officeExtension", "string"},
				{"officePhone", "string"},
				{"payrollIdentifier", "string"},
				{"payrollType", "int64"},
				{"resourceType", "string"},
				{"suffix", "int64"},
				{"surveyResourceRating", "float64"},
				{"timeFormat", "string"},
				{"title", "string"},
				{"travelAvailabilityPct", "string"},
				{"userName", "string"},
				{"userType", "int64"},
			},
		},
		{
			value: ResourceRole{}, wantEntity: "ResourceRoles", wantUDF: false,
			wantFields: []wantField{
				{"departmentID", "int64"},
				{"id", "int64"},
				{"isActive", "bool"},
				{"queueID", "int64"},
				{"resourceID", "int64"},
				{"roleID", "int64"},
			},
		},
		{
			value: ResourceRoleDepartment{}, wantEntity: "ResourceRoleDepartments", wantUDF: false,
			wantFields: []wantField{
				{"departmentID", "int64"},
				{"id", "int64"},
				{"isActive", "bool"},
				{"isDefault", "bool"},
				{"isDepartmentLead", "bool"},
				{"resourceID", "int64"},
				{"roleID", "int64"},
			},
		},
		{
			value: ResourceRoleQueue{}, wantEntity: "ResourceRoleQueues", wantUDF: false,
			wantFields: []wantField{
				{"id", "int64"},
				{"queueID", "int64"},
				{"resourceID", "int64"},
			},
		},
		{
			value: ResourceServiceDeskRole{}, wantEntity: "ResourceServiceDeskRoles", wantUDF: false,
			wantFields: []wantField{
				{"id", "int64"},
				{"isActive", "bool"},
				{"isDefault", "bool"},
				{"resourceID", "int64"},
				{"roleID", "int64"},
			},
		},
		{
			value: ResourceSkill{}, wantEntity: "ResourceSkills", wantUDF: false,
			wantFields: []wantField{
				{"id", "int64"},
				{"resourceID", "int64"},
				{"skillDescription", "string"},
				{"skillID", "int64"},
				{"skillLevel", "int64"},
			},
		},
		{
			value: ResourceAttachment{}, wantEntity: "ResourceAttachments", wantUDF: false,
			wantFields: []wantField{
				{"attachDate", "time.Time"},
				{"attachedByContactID", "int64"},
				{"attachedByResourceID", "int64"},
				{"attachmentType", "string"},
				{"contentType", "string"},
				{"creatorType", "int64"},
				{"data", "string"},
				{"fileSize", "int64"},
				{"fullPath", "string"},
				{"id", "int64"},
				{"impersonatorCreatorResourceID", "int64"},
				{"opportunityID", "int64"},
				{"parentID", "int64"},
				{"publish", "int64"},
				{"resourceID", "int64"},
				{"title", "string"},
			},
		},
		{
			value: Role{}, wantEntity: "Roles", wantUDF: false,
			wantFields: []wantField{
				{"description", "string"},
				{"hourlyFactor", "float64"},
				{"hourlyRate", "float64"},
				{"id", "int64"},
				{"isActive", "bool"},
				{"isExcludedFromNewContracts", "bool"},
				{"isSystemRole", "bool"},
				{"name", "string"},
				{"quoteItemDefaultTaxCategoryId", "int64"},
				{"roleType", "int64"},
			},
		},
		{
			value: Skill{}, wantEntity: "Skills", wantUDF: false,
			wantFields: []wantField{
				{"categoryID", "int64"},
				{"description", "string"},
				{"id", "int64"},
				{"isActive", "bool"},
				{"name", "string"},
			},
		},
		{
			value: ClientPortalUser{}, wantEntity: "ClientPortalUsers", wantUDF: false,
			wantFields: []wantField{
				{"contactID", "int64"},
				{"dateFormat", "int64"},
				{"id", "int64"},
				{"isClientPortalActive", "bool"},
				{"numberFormat", "int64"},
				{"password", "string"},
				{"securityLevel", "int64"},
				{"timeFormat", "int64"},
				{"userName", "string"},
			},
		},
		{
			value: ContactGroup{}, wantEntity: "ContactGroups", wantUDF: false,
			wantFields: []wantField{
				{"id", "int64"},
				{"isActive", "bool"},
				{"name", "string"},
			},
		},
		{
			value: ContactGroupContact{}, wantEntity: "ContactGroupContacts", wantUDF: false,
			wantFields: []wantField{
				{"contactGroupId", "int64"},
				{"contactId", "int64"},
				{"id", "int64"},
			},
		},
	}
	for _, tt := range tests {
		typ := reflect.TypeOf(tt.value)
		t.Run(typ.Name(), func(t *testing.T) {
			t.Parallel()
			if got := tt.value.EntityName(); got != tt.wantEntity {
				t.Errorf("EntityName() = %q, want %q", got, tt.wantEntity)
			}
			var got []wantField
			hasUDF := false
			for field := range typ.Fields() {
				if field.Name == "UserDefinedFields" {
					hasUDF = true
					continue
				}
				elem, ok := strings.CutPrefix(field.Type.Name(), "Optional[")
				elem, okEnd := strings.CutSuffix(elem, "]")
				if field.Type.PkgPath() != reflect.TypeFor[autotask.UDF]().PkgPath() || !ok || !okEnd {
					t.Errorf("%s has type %s, want autotask.Optional[...]", field.Name, field.Type)
					continue
				}
				jsonTag := field.Tag.Get("json")
				jsonName, opt, _ := strings.Cut(jsonTag, ",")
				if opt != "omitzero" {
					t.Errorf("%s has json tag %q, want the omitzero option", field.Name, jsonTag)
				}
				got = append(got, wantField{jsonName, elem})
			}
			if hasUDF != tt.wantUDF {
				t.Errorf("has UserDefinedFields = %v, want %v", hasUDF, tt.wantUDF)
			}
			if !slices.Equal(got, tt.wantFields) {
				t.Errorf("fields = %v, want %v", got, tt.wantFields)
			}
		})
	}
}
