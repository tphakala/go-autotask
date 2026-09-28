package autotask

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// GetRaw fetches one entity by id as an untyped map. Like Get, it returns a
// *NotFoundError when the id does not exist, whether the API answers with a
// 404 or with a 2xx whose item is missing or null.
func GetRaw(ctx context.Context, c *Client, entityName string, id int64) (map[string]any, error) {
	path := fmt.Sprintf("/v1.0/%s/%d", entityName, id)
	var resp struct {
		Item map[string]any `json:"item"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Item == nil {
		return nil, itemNotFoundError(entityName, id)
	}
	return resp.Item, nil
}

// ListRawIter returns an iterator over top-level entities (untyped) with lazy
// pagination. Ranging stops fetching further pages as soon as the caller
// breaks, so a caller that needs only the first few items does not pull every
// page up to maxPages. It is the untyped counterpart of ListIter.
//
// Like ListIter, ListRawIter does not enforce the query's MaxRecords cap
// client-side; the caller should stop ranging when it has enough items. ListRaw
// applies the MaxRecords cap on top of this iterator.
func ListRawIter(ctx context.Context, c *Client, entityName string, q *Query) iter.Seq2[map[string]any, error] {
	return func(yield func(map[string]any, error) bool) {
		path := fmt.Sprintf("/v1.0/%s/query", entityName)
		var queryBody any = q
		pages := 0
		for {
			pages++
			if pages > maxPages {
				yield(nil, &MaxPagesExceededError{EntityName: entityName, MaxPages: maxPages})
				return
			}
			nextPath, shouldContinue := fetchAndYieldRawPage(ctx, c, http.MethodPost, path, queryBody, yield)
			if !shouldContinue || nextPath == "" {
				return
			}
			path = nextPath
			queryBody = nil
		}
	}
}

// ListRaw fetches all top-level entities (untyped) matching the query, with
// automatic pagination. If the query sets MaxRecords, the result is capped to
// that many items. For large result sets, prefer ListRawIter to stop early
// instead of buffering every page.
func ListRaw(ctx context.Context, c *Client, entityName string, q *Query) ([]map[string]any, error) {
	totalLimit := 0
	if q != nil {
		totalLimit = q.MaxRecords()
	}
	var allItems []map[string]any
	for item, err := range ListRawIter(ctx, c, entityName, q) {
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, item)
		if totalLimit > 0 && len(allItems) >= totalLimit {
			break
		}
	}
	return allItems, nil
}

func CreateRaw(ctx context.Context, c *Client, entityName string, data map[string]any) (map[string]any, error) {
	path := fmt.Sprintf("/v1.0/%s", entityName)
	var resp map[string]any
	if err := c.do(ctx, http.MethodPost, path, data, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// UpdateRaw patches an entity from an untyped map. Like CreateRaw, it returns
// the decoded response body as is, for example {"itemId": N}.
func UpdateRaw(ctx context.Context, c *Client, entityName string, data map[string]any) (map[string]any, error) {
	path := fmt.Sprintf("/v1.0/%s", entityName)
	var resp map[string]any
	if err := c.do(ctx, http.MethodPatch, path, data, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func DeleteRaw(ctx context.Context, c *Client, entityName string, id int64) error {
	path := fmt.Sprintf("/v1.0/%s/%d", entityName, id)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
