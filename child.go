package autotask

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

// resolveChildName returns the URL segment for a child entity.
// If the entity implements ChildEntity, its ChildEntityName is used;
// otherwise it falls back to EntityName.
func resolveChildName[C Entity](c C) string {
	if ce, ok := any(c).(ChildEntity); ok {
		return ce.ChildEntityName()
	}
	return c.EntityName()
}

// Deprecated: Use ListChild which provides automatic pagination.
// GetChild fetches child entities for a parent entity (first page only).
func GetChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64) ([]*C, error) {
	var parent P
	var child C
	path := fmt.Sprintf("/v1.0/%s/%d/%s", parent.EntityName(), parentID, resolveChildName(child))
	var resp pageResponse[json.RawMessage]
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
	var allItems []*C
	for entity, err := range ListChildIter[P, C](ctx, c, parentID) {
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, entity)
	}
	return allItems, nil
}

// ListChildIter returns an iterator over child entities with lazy pagination.
func ListChildIter[P Entity, C Entity](ctx context.Context, c *Client, parentID int64) iter.Seq2[*C, error] {
	return func(yield func(*C, error) bool) {
		var zeroP P
		var zeroC C
		path := fmt.Sprintf("/v1.0/%s/%d/%s", zeroP.EntityName(), parentID, resolveChildName(zeroC))
		label := zeroC.EntityName() + " child"
		pages := 0
		for {
			pages++
			if pages > maxPages {
				yield(nil, &MaxPagesExceededError{EntityName: zeroC.EntityName(), MaxPages: maxPages})
				return
			}
			nextPath, shouldContinue := fetchAndYieldTypedPage(ctx, c, http.MethodGet, path, nil, label, yield)
			if !shouldContinue || nextPath == "" {
				return
			}
			path = nextPath
		}
	}
}

// ListChildRawIter returns an iterator over child entities (untyped) for a
// parent with lazy pagination. Ranging stops fetching further pages as soon as
// the caller breaks, so a caller that needs only the first few items does not
// pull every page up to maxPages (including large blob fields such as an
// attachment's data). It is the untyped counterpart of ListChildIter.
func ListChildRawIter(ctx context.Context, c *Client, parentEntityName string, parentID int64, childEntityName string) iter.Seq2[map[string]any, error] {
	return func(yield func(map[string]any, error) bool) {
		path := fmt.Sprintf("/v1.0/%s/%d/%s", parentEntityName, parentID, childEntityName)
		pages := 0
		for {
			pages++
			if pages > maxPages {
				yield(nil, &MaxPagesExceededError{EntityName: childEntityName, MaxPages: maxPages})
				return
			}
			nextPath, shouldContinue := fetchAndYieldRawPage(ctx, c, http.MethodGet, path, nil, yield)
			if !shouldContinue || nextPath == "" {
				return
			}
			path = nextPath
		}
	}
}

// ListChildRaw fetches all child entities (untyped) for a parent, with automatic
// pagination. For large or blob-heavy collections, prefer ListChildRawIter to
// stop early instead of buffering every page.
func ListChildRaw(ctx context.Context, c *Client, parentEntityName string, parentID int64, childEntityName string) ([]map[string]any, error) {
	var allItems []map[string]any
	for item, err := range ListChildRawIter(ctx, c, parentEntityName, parentID, childEntityName) {
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, item)
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
	path := fmt.Sprintf("/v1.0/%s/%d/%s", parent.EntityName(), parentID, resolveChildName(*child))
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
