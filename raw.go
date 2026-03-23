package autotask

import (
	"context"
	"fmt"
	"net/http"
)

func GetRaw(ctx context.Context, c *Client, entityName string, id int64) (map[string]any, error) {
	path := fmt.Sprintf("/v1.0/%s/%d", entityName, id)
	var resp struct {
		Item map[string]any `json:"item"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Item, nil
}

func ListRaw(ctx context.Context, c *Client, entityName string, q *Query) ([]map[string]any, error) {
	path := fmt.Sprintf("/v1.0/%s/query", entityName)
	totalLimit := 0
	if q != nil {
		totalLimit = q.MaxRecords()
	}
	var allItems []map[string]any
	var queryBody any = q
	for {
		var resp struct {
			Items       []map[string]any `json:"items"`
			PageDetails struct {
				Count       int    `json:"count"`
				NextPageURL string `json:"nextPageUrl"`
			} `json:"pageDetails"`
		}
		if err := c.do(ctx, http.MethodPost, path, queryBody, &resp); err != nil {
			return nil, err
		}
		allItems = append(allItems, resp.Items...)
		if totalLimit > 0 && len(allItems) >= totalLimit {
			allItems = allItems[:totalLimit]
			break
		}
		if resp.PageDetails.NextPageURL == "" {
			break
		}
		path = resp.PageDetails.NextPageURL
		queryBody = nil
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

func UpdateRaw(ctx context.Context, c *Client, entityName string, data map[string]any) (map[string]any, error) {
	path := fmt.Sprintf("/v1.0/%s", entityName)
	var resp struct {
		Item map[string]any `json:"item"`
	}
	if err := c.do(ctx, http.MethodPatch, path, data, &resp); err != nil {
		return nil, err
	}
	return resp.Item, nil
}

func DeleteRaw(ctx context.Context, c *Client, entityName string, id int64) error {
	path := fmt.Sprintf("/v1.0/%s/%d", entityName, id)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
