package autotask

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GetChild fetches child entities for a parent entity (first page only).
func GetChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64) ([]*C, error) {
	var parent P
	var child C
	path := fmt.Sprintf("/v1.0/%s/%d/%s", parent.EntityName(), parentID, child.EntityName())
	var resp struct {
		Items       []json.RawMessage `json:"items"`
		PageDetails struct {
			NextPageURL string `json:"nextPageUrl"`
		} `json:"pageDetails"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	var result []*C
	for _, raw := range resp.Items {
		var entity C
		if err := json.Unmarshal(raw, &entity); err != nil {
			return nil, fmt.Errorf("autotask: decoding %s child: %w", child.EntityName(), err)
		}
		result = append(result, &entity)
	}
	return result, nil
}

// CreateChild creates a child entity under a parent.
func CreateChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64, child *C) (*C, error) {
	if child == nil {
		return nil, fmt.Errorf("autotask: child entity must not be nil")
	}
	var parent P
	path := fmt.Sprintf("/v1.0/%s/%d/%s", parent.EntityName(), parentID, (*child).EntityName())
	var resp json.RawMessage
	if err := c.do(ctx, http.MethodPost, path, child, &resp); err != nil {
		return nil, err
	}
	return child, nil
}
