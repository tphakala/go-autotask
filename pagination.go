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
	Items       pageItems[I] `json:"items"`
	PageDetails struct {
		NextPageURL string `json:"nextPageUrl"`
	} `json:"pageDetails"`
}

// pageItems decodes the items member of a page and records that the member was
// present. encoding/json calls UnmarshalJSON only when the member exists, null
// included, and not when it is absent or when the whole body is null (measured
// with go1.27.4; TestListPageWithoutItemsIsMissingItemsError pins it). That
// tells an absent member from an empty page, which a plain slice cannot.
type pageItems[I any] struct {
	present bool
	list    []I
}

func (p *pageItems[I]) UnmarshalJSON(data []byte) error {
	p.present = true
	return json.Unmarshal(data, &p.list)
}

// fetchPage sends one list request and decodes the page. It returns
// *MissingItemsError when the response has no items member, so that such a body
// is not read as an empty page. It returns the next page URL (empty when there
// are no further pages).
func fetchPage[I any](ctx context.Context, c *Client, method, path string, body any) (items []I, nextPageURL string, err error) {
	var resp pageResponse[I]
	if err := c.do(ctx, method, path, body, &resp); err != nil {
		return nil, "", err
	}
	if !resp.Items.present {
		return nil, "", newMissingItemsError()
	}
	return resp.Items.list, resp.PageDetails.NextPageURL, nil
}

// fetchAndYieldTypedPage fetches one page from path, decodes it, unmarshals each
// item into T, and yields the results. decodeLabel names the entity in
// decode-error messages (for example "Ticket" for a top-level entity or
// "Notes child" for a child entity). It returns the next page URL (empty when
// there are no further pages) and whether iteration should continue: false once
// yield returns false or a request or page error has been yielded. It backs both the
// top-level and child typed iterators.
func fetchAndYieldTypedPage[T Entity](ctx context.Context, c *Client, method, path string, body any, decodeLabel string, yield func(*T, error) bool) (string, bool) {
	items, nextPageURL, err := fetchPage[json.RawMessage](ctx, c, method, path, body)
	if err != nil {
		yield(nil, err)
		return "", false
	}
	for _, raw := range items {
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
	return nextPageURL, true
}

// fetchAndYieldRawPage fetches one untyped page from path and yields each item
// as a map, without unmarshalling into a concrete type. It returns the next page
// URL (empty when there are no further pages) and whether iteration should
// continue (false once yield returns false or a request or page error has been
// yielded). It backs both the top-level and child raw iterators.
func fetchAndYieldRawPage(ctx context.Context, c *Client, method, path string, body any, yield func(map[string]any, error) bool) (string, bool) {
	items, nextPageURL, err := fetchPage[map[string]any](ctx, c, method, path, body)
	if err != nil {
		yield(nil, err)
		return "", false
	}
	for _, item := range items {
		if !yield(item, nil) {
			return "", false
		}
	}
	return nextPageURL, true
}
