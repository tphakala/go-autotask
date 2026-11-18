package autotask

import (
	"context"
	"errors"
	"iter"
	"net/http"
	"strings"
	"testing"
)

// drain ranges over seq and returns the number of items before the first
// error, as a ranging caller would.
func drain[T any](seq iter.Seq2[T, error]) (int, error) {
	n := 0
	for _, err := range seq {
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func count[T any](items []T, err error) (int, error) { return len(items), err }

// listCalls are the public calls that decode a list page, each reduced to the
// number of items it returned and its error.
func listCalls() map[string]func(context.Context, *Client) (int, error) {
	return map[string]func(context.Context, *Client) (int, error){
		"List": func(ctx context.Context, c *Client) (int, error) {
			return count(List[testEntity](ctx, c, &Query{}))
		},
		"ListIter": func(ctx context.Context, c *Client) (int, error) {
			return drain(ListIter[testEntity](ctx, c, &Query{}))
		},
		"ListRaw": func(ctx context.Context, c *Client) (int, error) {
			return count(ListRaw(ctx, c, "TestEntities", &Query{}))
		},
		"ListRawIter": func(ctx context.Context, c *Client) (int, error) {
			return drain(ListRawIter(ctx, c, "TestEntities", &Query{}))
		},
		"ListChild": func(ctx context.Context, c *Client) (int, error) {
			return count(ListChild[testEntity, testChildEntity](ctx, c, 1))
		},
		"ListChildIter": func(ctx context.Context, c *Client) (int, error) {
			return drain(ListChildIter[testEntity, testChildEntity](ctx, c, 1))
		},
		"ListChildRaw": func(ctx context.Context, c *Client) (int, error) {
			return count(ListChildRaw(ctx, c, "TestEntities", 1, "Notes"))
		},
		"ListChildRawIter": func(ctx context.Context, c *Client) (int, error) {
			return drain(ListChildRawIter(ctx, c, "TestEntities", 1, "Notes"))
		},
		"GetChild": func(ctx context.Context, c *Client) (int, error) {
			return count(GetChild[testEntity, testChildEntity](ctx, c, 1))
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
