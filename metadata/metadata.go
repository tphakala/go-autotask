package metadata

import (
	"context"
	"fmt"
	"net/http"

	autotask "github.com/tphakala/go-autotask"
)

type FieldInfo struct {
	Name           string          `json:"name"`
	Label          string          `json:"label"`
	Type           string          `json:"dataType"`
	IsRequired     bool            `json:"isRequired"`
	IsReadOnly     bool            `json:"isReadOnly"`
	IsPickList     bool            `json:"isPickList"`
	PickListValues []PickListValue `json:"picklistValues,omitempty"`
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

func GetFields(ctx context.Context, c *autotask.Client, entityName string) ([]FieldInfo, error) {
	path := fmt.Sprintf("/v1.0/%s/entityInformation/fields", entityName)
	var resp struct {
		Fields []FieldInfo `json:"fields"`
	}
	if err := c.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Fields, nil
}

func GetUDFs(ctx context.Context, c *autotask.Client, entityName string) ([]UDFInfo, error) {
	path := fmt.Sprintf("/v1.0/%s/entityInformation/userDefinedFields", entityName)
	var resp struct {
		Fields []UDFInfo `json:"fields"`
	}
	if err := c.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Fields, nil
}

func GetEntityInfo(ctx context.Context, c *autotask.Client, entityName string) (*EntityInfo, error) {
	path := fmt.Sprintf("/v1.0/%s/entityInformation", entityName)
	var resp struct {
		Info EntityInfo `json:"info"`
	}
	if err := c.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Info, nil
}

// GetPickList returns the picklist values for a specific field on an entity.
// Returns an error if the field is not found or is not a picklist field.
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
