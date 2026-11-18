package autotask

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// listCalls are the public calls that decode a list page, each reduced to the
// number of items it returned and its error. The iterator rows stop at the
// first error, as a ranging caller would.
func listCalls() map[string]func(context.Context, *Client) (int, error) {
	return map[string]func(context.Context, *Client) (int, error){
		"List": func(ctx context.Context, c *Client) (int, error) {
			items, err := List[testEntity](ctx, c, &Query{})
			return len(items), err
		},
		"ListIter": func(ctx context.Context, c *Client) (int, error) {
			n := 0
			for _, err := range ListIter[testEntity](ctx, c, &Query{}) {
				if err != nil {
					return n, err
				}
				n++
			}
			return n, nil
		},
		"ListRaw": func(ctx context.Context, c *Client) (int, error) {
			items, err := ListRaw(ctx, c, "TestEntities", &Query{})
			return len(items), err
		},
		"ListRawIter": func(ctx context.Context, c *Client) (int, error) {
			n := 0
			for _, err := range ListRawIter(ctx, c, "TestEntities", &Query{}) {
				if err != nil {
					return n, err
				}
				n++
			}
			return n, nil
		},
		"ListChild": func(ctx context.Context, c *Client) (int, error) {
			items, err := ListChild[testEntity, testChildEntity](ctx, c, 1)
			return len(items), err
		},
		"ListChildIter": func(ctx context.Context, c *Client) (int, error) {
			n := 0
			for _, err := range ListChildIter[testEntity, testChildEntity](ctx, c, 1) {
				if err != nil {
					return n, err
				}
				n++
			}
			return n, nil
		},
		"ListChildRaw": func(ctx context.Context, c *Client) (int, error) {
			items, err := ListChildRaw(ctx, c, "TestEntities", 1, "Notes")
			return len(items), err
		},
		"ListChildRawIter": func(ctx context.Context, c *Client) (int, error) {
			n := 0
			for _, err := range ListChildRawIter(ctx, c, "TestEntities", 1, "Notes") {
				if err != nil {
					return n, err
				}
				n++
			}
			return n, nil
		},
		"GetChild": func(ctx context.Context, c *Client) (int, error) {
			items, err := GetChild[testEntity, testChildEntity](ctx, c, 1)
			return len(items), err
		},
	}
}

func TestListPageWithoutItemsIsMissingItemsError(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{}`, `null`, `{"pageDetails":{"nextPageUrl":null}}`} {
		for name, call := range listCalls() {
			t.Run(name+"/"+body, func(t *testing.T) {
				t.Parallel()
				c := newStaticClient(t, http.StatusOK, body)
				n, err := call(t.Context(), c)
				mi, ok := errors.AsType[*MissingItemsError](err)
				if !ok {
					t.Fatalf("err = %T %v; want *MissingItemsError", err, err)
				}
				if mi.Err.StatusCode != http.StatusOK {
					t.Errorf("Err.StatusCode = %d; want 200", mi.Err.StatusCode)
				}
				if !strings.Contains(err.Error(), "items") {
					t.Errorf("message %q does not name the missing items member", err.Error())
				}
				if _, ok := errors.AsType[*Error](err); !ok {
					t.Errorf("errors.As(%T, *Error) = false; want true", err)
				}
				if _, ok := errors.AsType[*NotFoundError](err); ok {
					t.Errorf("%T matches *NotFoundError; a page without items is not a missing record", err)
				}
				if n != 0 {
					t.Errorf("returned %d items beside the error; want 0", n)
				}
			})
		}
	}
}

func TestListPageWithEmptyOrNullItemsIsEmpty(t *testing.T) {
	t.Parallel()
	bodies := []string{
		`{"items":[],"pageDetails":{"count":0,"requestCount":1,"prevPageUrl":null,"nextPageUrl":null}}`,
		`{"items":null,"pageDetails":{"nextPageUrl":null}}`,
	}
	for _, body := range bodies {
		for name, call := range listCalls() {
			t.Run(name+"/"+body, func(t *testing.T) {
				t.Parallel()
				c := newStaticClient(t, http.StatusOK, body)
				n, err := call(t.Context(), c)
				if err != nil {
					t.Fatalf("err = %v; want nil", err)
				}
				if n != 0 {
					t.Errorf("returned %d items; want 0", n)
				}
			})
		}
	}
}
