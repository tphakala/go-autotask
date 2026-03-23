package autotask

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

func ListIter[T Entity](ctx context.Context, c *Client, q *Query) iter.Seq2[*T, error] {
	return func(yield func(*T, error) bool) {
		var zero T
		path := fmt.Sprintf("/v1.0/%s/query", zero.EntityName())
		var queryBody any = q
		for {
			var resp struct {
				Items       []json.RawMessage `json:"items"`
				PageDetails struct {
					Count       int    `json:"count"`
					NextPageURL string `json:"nextPageUrl"`
				} `json:"pageDetails"`
			}
			if err := c.do(ctx, http.MethodPost, path, queryBody, &resp); err != nil {
				if !yield(nil, err) {
					return
				}
				return
			}
			for _, raw := range resp.Items {
				var entity T
				if err := json.Unmarshal(raw, &entity); err != nil {
					if !yield(nil, fmt.Errorf("autotask: decoding %s: %w", zero.EntityName(), err)) {
						return
					}
					continue
				}
				if !yield(&entity, nil) {
					return
				}
			}
			if resp.PageDetails.NextPageURL == "" {
				return
			}
			path = resp.PageDetails.NextPageURL
			queryBody = nil
		}
	}
}
