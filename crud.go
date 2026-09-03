package autotask

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func Get[T Entity](ctx context.Context, c *Client, id int64) (*T, error) {
	var zero T
	path := fmt.Sprintf("/v1.0/%s/%d", zero.EntityName(), id)
	var resp struct {
		Item json.RawMessage `json:"item"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Item == nil || string(resp.Item) == "null" {
		return nil, fmt.Errorf("autotask: %s %d returned no item", zero.EntityName(), id)
	}
	var entity T
	if err := json.Unmarshal(resp.Item, &entity); err != nil {
		return nil, fmt.Errorf("autotask: decoding %s: %w", zero.EntityName(), err)
	}
	return &entity, nil
}

func List[T Entity](ctx context.Context, c *Client, q *Query) ([]*T, error) {
	totalLimit := 0
	if q != nil {
		totalLimit = q.MaxRecords()
	}
	var allItems []*T
	for entity, err := range ListIter[T](ctx, c, q) {
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, entity)
		if totalLimit > 0 && len(allItems) >= totalLimit {
			break
		}
	}
	return allItems, nil
}

func Count[T Entity](ctx context.Context, c *Client, q *Query) (int64, error) {
	var zero T
	path := fmt.Sprintf("/v1.0/%s/query/count", zero.EntityName())
	var resp struct {
		QueryCount int64 `json:"queryCount"`
	}
	if err := c.do(ctx, http.MethodPost, path, q, &resp); err != nil {
		return 0, err
	}
	return resp.QueryCount, nil
}

func Create[T Entity](ctx context.Context, c *Client, entity *T) (*T, error) {
	if entity == nil {
		return nil, fmt.Errorf("autotask: entity must not be nil")
	}
	path := fmt.Sprintf("/v1.0/%s", (*entity).EntityName())
	var resp struct {
		ItemID *int64 `json:"itemId"`
	}
	if err := c.do(ctx, http.MethodPost, path, entity, &resp); err != nil {
		return nil, err
	}
	if resp.ItemID != nil {
		if setter, ok := any(entity).(EntityWithID); ok {
			setter.SetID(*resp.ItemID)
		}
	}
	return entity, nil
}

func Update[T Entity](ctx context.Context, c *Client, entity *T) (*T, error) {
	if entity == nil {
		return nil, fmt.Errorf("autotask: entity must not be nil")
	}
	path := fmt.Sprintf("/v1.0/%s", (*entity).EntityName())
	var resp struct {
		Item json.RawMessage `json:"item"`
	}
	if err := c.do(ctx, http.MethodPatch, path, entity, &resp); err != nil {
		return nil, err
	}
	if resp.Item != nil {
		var updated T
		if err := json.Unmarshal(resp.Item, &updated); err != nil {
			return nil, fmt.Errorf("autotask: decoding updated %s: %w", (*entity).EntityName(), err)
		}
		return &updated, nil
	}
	return entity, nil
}

func Delete[T Entity](ctx context.Context, c *Client, id int64) error {
	var zero T
	path := fmt.Sprintf("/v1.0/%s/%d", zero.EntityName(), id)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
