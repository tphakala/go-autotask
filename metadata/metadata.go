package metadata

import (
	"context"
	"fmt"
	"net/http"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/internal/jsonmember"
)

type FieldInfo struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Type  string `json:"dataType"`
	// Length is the maximum length of a string value, or 0 when Autotask
	// reports none.
	Length      int  `json:"length,omitempty"`
	IsRequired  bool `json:"isRequired"`
	IsReadOnly  bool `json:"isReadOnly"`
	IsQueryable bool `json:"isQueryable"`
	IsPickList  bool `json:"isPickList"`
	// IsReference marks a field that holds the ID of another entity, named by
	// ReferenceEntityType.
	IsReference         bool            `json:"isReference"`
	ReferenceEntityType string          `json:"referenceEntityType,omitempty"`
	PickListValues      []PickListValue `json:"picklistValues,omitempty"`
}

type PickListValue struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	IsActive    bool   `json:"isActive"`
	SortOrder   int    `json:"sortOrder,omitempty"`
	ParentValue string `json:"parentValue,omitempty"`
	IsSystem    bool   `json:"isSystem,omitempty"`
}

type UDFInfo struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	Type       string `json:"dataType"`
	IsRequired bool   `json:"isRequired"`
}

type EntityInfo struct {
	Name                 string `json:"name"`
	CanCreate            bool   `json:"canCreate"`
	CanUpdate            bool   `json:"canUpdate"`
	CanDelete            bool   `json:"canDelete"`
	CanQuery             bool   `json:"canQuery"`
	HasUserDefinedFields bool   `json:"hasUserDefinedFields"`
}

// missingMember is the error for a successful response whose JSON body lacks
// the member that holds the result.
func missingMember(member string) *autotask.MissingMemberError {
	return &autotask.MissingMemberError{
		Err:    autotask.Error{StatusCode: http.StatusOK, Message: "response has no " + member + " member"},
		Member: member,
	}
}

// GetFields returns the fields of entityName. It returns
// *autotask.MissingMemberError when the response has no fields member, such as
// {}; "fields": [] and "fields": null are an empty list.
func GetFields(ctx context.Context, c *autotask.Client, entityName string) ([]FieldInfo, error) {
	path := fmt.Sprintf("/v1.0/%s/entityInformation/fields", entityName)
	var resp struct {
		Fields jsonmember.Member[[]FieldInfo] `json:"fields"`
	}
	if err := c.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	if !resp.Fields.Present {
		return nil, missingMember("fields")
	}
	return resp.Fields.Value, nil
}

// GetUDFs returns the user-defined fields of entityName. It returns
// *autotask.MissingMemberError when the response has no fields member, such as
// {}; "fields": [] and "fields": null are an empty list.
func GetUDFs(ctx context.Context, c *autotask.Client, entityName string) ([]UDFInfo, error) {
	path := fmt.Sprintf("/v1.0/%s/entityInformation/userDefinedFields", entityName)
	var resp struct {
		Fields jsonmember.Member[[]UDFInfo] `json:"fields"`
	}
	if err := c.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	if !resp.Fields.Present {
		return nil, missingMember("fields")
	}
	return resp.Fields.Value, nil
}

// GetEntityInfo returns what Autotask reports about entityName. It returns
// *autotask.MissingMemberError when the response has no info member, such as
// {}, or has "info": null; "info": {} is a valid EntityInfo with every
// capability false.
func GetEntityInfo(ctx context.Context, c *autotask.Client, entityName string) (*EntityInfo, error) {
	path := fmt.Sprintf("/v1.0/%s/entityInformation", entityName)
	var resp struct {
		Info *EntityInfo `json:"info"`
	}
	if err := c.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Info == nil {
		return nil, missingMember("info")
	}
	return resp.Info, nil
}

// GetPickList returns the picklist values for a specific field on an entity.
// Returns an error if the field is not found or is not a picklist field. An
// error from GetFields is returned as is.
func GetPickList(ctx context.Context, c *autotask.Client, entityName, fieldName string) ([]PickListValue, error) {
	fields, err := GetFields(ctx, c, entityName)
	if err != nil {
		return nil, err
	}
	for _, f := range fields {
		if f.Name == fieldName {
			if !f.IsPickList {
				return nil, fmt.Errorf("metadata: field %q on %s is not a picklist field", fieldName, entityName)
			}
			return f.PickListValues, nil
		}
	}
	return nil, fmt.Errorf("metadata: field %q not found on %s", fieldName, entityName)
}
