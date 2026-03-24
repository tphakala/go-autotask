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
		pages := 0
		for {
			pages++
			if pages > maxPages {
				yield(nil, &ErrMaxPagesExceeded{EntityName: zero.EntityName(), MaxPages: maxPages})
				return
			}
			nextPath, shouldContinue := fetchAndYieldPage(ctx, c, &zero, path, queryBody, yield)
			if !shouldContinue {
				return
			}
			if nextPath == "" {
				return
			}
			path = nextPath
			queryBody = nil
		}
	}
}

func fetchAndYieldPage[T Entity](ctx context.Context, c *Client, entityZero *T, path string, queryBody any, yield func(*T, error) bool) (string, bool) {
	var resp struct {
		Items       []json.RawMessage `json:"items"`
		PageDetails struct {
			Count       int    `json:"count"`
			NextPageURL string `json:"nextPageUrl"`
		} `json:"pageDetails"`
	}
	if err := c.do(ctx, http.MethodPost, path, queryBody, &resp); err != nil {
		yield(nil, err)
		return "", false
	}
	for _, raw := range resp.Items {
		var entity T
		if err := json.Unmarshal(raw, &entity); err != nil {
			if !yield(nil, fmt.Errorf("autotask: decoding %s: %w", (*entityZero).EntityName(), err)) {
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
