package autotask

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

type childPageResponse struct {
	Items       []json.RawMessage `json:"items"`
	PageDetails struct {
		NextPageURL string `json:"nextPageUrl"`
	} `json:"pageDetails"`
}

// Deprecated: Use ListChild which provides automatic pagination.
// GetChild fetches child entities for a parent entity (first page only).
func GetChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64) ([]*C, error) {
	var parent P
	var child C
	path := fmt.Sprintf("/v1.0/%s/%d/%s", parent.EntityName(), parentID, child.EntityName())
	var resp childPageResponse
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

// ListChild fetches all child entities for a parent, with automatic pagination.
func ListChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64) ([]*C, error) {
	var zeroP P
	var zeroC C
	path := fmt.Sprintf("/v1.0/%s/%d/%s", zeroP.EntityName(), parentID, zeroC.EntityName())
	var allItems []*C
	pages := 0
	for {
		pages++
		if pages > maxPages {
			return nil, &MaxPagesExceededError{EntityName: zeroC.EntityName(), MaxPages: maxPages}
		}
		var resp childPageResponse
		if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return nil, err
		}
		for _, raw := range resp.Items {
			var entity C
			if err := json.Unmarshal(raw, &entity); err != nil {
				return nil, fmt.Errorf("autotask: decoding %s child: %w", zeroC.EntityName(), err)
			}
			allItems = append(allItems, &entity)
		}
		if resp.PageDetails.NextPageURL == "" {
			break
		}
		path = resp.PageDetails.NextPageURL
	}
	return allItems, nil
}

// ListChildIter returns an iterator over child entities with lazy pagination.
func ListChildIter[P Entity, C Entity](ctx context.Context, c *Client, parentID int64) iter.Seq2[*C, error] {
	return func(yield func(*C, error) bool) {
		var zeroP P
		var zeroC C
		path := fmt.Sprintf("/v1.0/%s/%d/%s", zeroP.EntityName(), parentID, zeroC.EntityName())
		pages := 0
		for {
			pages++
			if pages > maxPages {
				yield(nil, &MaxPagesExceededError{EntityName: zeroC.EntityName(), MaxPages: maxPages})
				return
			}
			nextPath, shouldContinue := fetchAndYieldChildPage(ctx, c, &zeroC, path, yield)
			if !shouldContinue || nextPath == "" {
				return
			}
			path = nextPath
		}
	}
}

func fetchAndYieldChildPage[C Entity](ctx context.Context, c *Client, entityZero *C, path string, yield func(*C, error) bool) (string, bool) {
	var resp childPageResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		yield(nil, err)
		return "", false
	}
	for _, raw := range resp.Items {
		var entity C
		if err := json.Unmarshal(raw, &entity); err != nil {
			if !yield(nil, fmt.Errorf("autotask: decoding %s child: %w", (*entityZero).EntityName(), err)) {
				return "", false
			}
			continue
		}
		if !yield(&entity, nil) {
			return "", false
		}
	}
	return resp.PageDetails.NextPageURL, true
}

// ListChildRaw fetches all child entities (untyped) for a parent, with automatic pagination.
func ListChildRaw(ctx context.Context, c *Client, parentEntityName string, parentID int64, childEntityName string) ([]map[string]any, error) {
	path := fmt.Sprintf("/v1.0/%s/%d/%s", parentEntityName, parentID, childEntityName)
	var allItems []map[string]any
	pages := 0
	for {
		pages++
		if pages > maxPages {
			return nil, &MaxPagesExceededError{EntityName: childEntityName, MaxPages: maxPages}
		}
		var resp struct {
			Items       []map[string]any `json:"items"`
			PageDetails struct {
				NextPageURL string `json:"nextPageUrl"`
			} `json:"pageDetails"`
		}
		if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return nil, err
		}
		allItems = append(allItems, resp.Items...)
		if resp.PageDetails.NextPageURL == "" {
			break
		}
		path = resp.PageDetails.NextPageURL
	}
	return allItems, nil
}

// CreateChildRaw creates a child entity (untyped) under a parent.
func CreateChildRaw(ctx context.Context, c *Client, parentEntityName string, parentID int64, childEntityName string, data map[string]any) (map[string]any, error) {
	path := fmt.Sprintf("/v1.0/%s/%d/%s", parentEntityName, parentID, childEntityName)
	var resp map[string]any
	if err := c.do(ctx, http.MethodPost, path, data, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateChild creates a child entity under a parent.
func CreateChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64, child *C) (*C, error) {
	if child == nil {
		return nil, fmt.Errorf("autotask: child entity must not be nil")
	}
	var parent P
	path := fmt.Sprintf("/v1.0/%s/%d/%s", parent.EntityName(), parentID, (*child).EntityName())
	var resp struct {
		ItemID *int64 `json:"itemId"`
	}
	if err := c.do(ctx, http.MethodPost, path, child, &resp); err != nil {
		return nil, err
	}
	if resp.ItemID != nil {
		if setter, ok := any(child).(EntityWithID); ok {
			setter.SetID(*resp.ItemID)
		}
	}
	return child, nil
}
