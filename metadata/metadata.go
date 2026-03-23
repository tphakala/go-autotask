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
	Value    int    `json:"value"`
	Label    string `json:"label"`
	IsActive bool   `json:"isActive"`
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
	var info EntityInfo
	if err := c.Do(ctx, http.MethodGet, path, nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
