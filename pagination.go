package autotask

import (
	"context"
	"encoding/json"
	"fmt"
)

// maxPages is the safety limit on pagination loops to prevent infinite loops
// from malformed API responses that cycle nextPageUrl values.
const maxPages = 1000

// MaxPagesExceededError is returned when a pagination loop exceeds maxPages.
type MaxPagesExceededError struct {
	EntityName string
	MaxPages   int
}

func (e *MaxPagesExceededError) Error() string {
	return fmt.Sprintf("autotask: exceeded maximum page limit (%d) fetching %s", e.MaxPages, e.EntityName)
}

// pageResponse decodes one page of a list/query response. The element type I is
// json.RawMessage for the typed listers, which unmarshal each item into a
// concrete entity, and map[string]any for the untyped raw listers.
type pageResponse[I any] struct {
	Items       []I `json:"items"`
	PageDetails struct {
		NextPageURL string `json:"nextPageUrl"`
	} `json:"pageDetails"`
}

// fetchAndYieldTypedPage fetches one page from path, decodes it, unmarshals each
// item into T, and yields the results. decodeLabel names the entity in
// decode-error messages (for example "Ticket" for a top-level entity or
// "Notes child" for a child entity). It returns the next page URL (empty when
// there are no further pages) and whether iteration should continue: false once
// yield returns false or a request error has been yielded. It backs both the
// top-level and child typed iterators.
func fetchAndYieldTypedPage[T Entity](ctx context.Context, c *Client, method, path string, body any, decodeLabel string, yield func(*T, error) bool) (string, bool) {
	var resp pageResponse[json.RawMessage]
	if err := c.do(ctx, method, path, body, &resp); err != nil {
		yield(nil, err)
		return "", false
	}
	for _, raw := range resp.Items {
		var entity T
		if err := json.Unmarshal(raw, &entity); err != nil {
			if !yield(nil, fmt.Errorf("autotask: decoding %s: %w", decodeLabel, err)) {
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

// fetchAndYieldRawPage fetches one untyped page from path and yields each item
// as a map, without unmarshalling into a concrete type. It returns the next page
// URL (empty when there are no further pages) and whether iteration should
// continue. It backs both the top-level and child raw iterators.
func fetchAndYieldRawPage(ctx context.Context, c *Client, method, path string, body any, yield func(map[string]any, error) bool) (string, bool) {
	var resp pageResponse[map[string]any]
	if err := c.do(ctx, method, path, body, &resp); err != nil {
		yield(nil, err)
		return "", false
	}
	for _, item := range resp.Items {
		if !yield(item, nil) {
			return "", false
		}
	}
	return resp.PageDetails.NextPageURL, true
}
