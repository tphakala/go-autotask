package autotask

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// ListIter returns an iterator over top-level entities with lazy pagination.
// Every page is requested by POST with the same query, since Autotask paging
// URLs need the original query. The query is encoded once per range, so changing
// q while ranging does not change later pages.
func ListIter[T Entity](ctx context.Context, c *Client, q *Query) iter.Seq2[*T, error] {
	return func(yield func(*T, error) bool) {
		var zero T
		path := fmt.Sprintf("/v1.0/%s/query", zero.EntityName())
		query, err := encodeQuery(q)
		if err != nil {
			yield(nil, err)
			return
		}
		label := zero.EntityName()
		pages := 0
		for {
			pages++
			if pages > maxPages {
				yield(nil, &MaxPagesExceededError{EntityName: zero.EntityName(), MaxPages: maxPages})
				return
			}
			nextPath, shouldContinue := fetchAndYieldTypedPage(ctx, c, http.MethodPost, path, c.queryPageBody(path, query), label, yield)
			if !shouldContinue || nextPath == "" {
				return
			}
			path = nextPath
		}
	}
}
