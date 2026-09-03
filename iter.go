package autotask

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

func ListIter[T Entity](ctx context.Context, c *Client, q *Query) iter.Seq2[*T, error] {
	return func(yield func(*T, error) bool) {
		var zero T
		path := fmt.Sprintf("/v1.0/%s/query", zero.EntityName())
		var queryBody any = q
		label := zero.EntityName()
		pages := 0
		for {
			pages++
			if pages > maxPages {
				yield(nil, &MaxPagesExceededError{EntityName: zero.EntityName(), MaxPages: maxPages})
				return
			}
			nextPath, shouldContinue := fetchAndYieldTypedPage(ctx, c, http.MethodPost, path, queryBody, label, yield)
			if !shouldContinue || nextPath == "" {
				return
			}
			path = nextPath
			queryBody = nil
		}
	}
}
