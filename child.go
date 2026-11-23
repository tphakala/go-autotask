package autotask

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

// childPath builds the collection URL of a child entity under its parent:
// /v1.0/{parent}/{parentID}/{child}. Every child call goes through it, so the
// same (P, C) pair addresses the same collection for list, create, update and
// delete.
func childPath(parentName string, parentID int64, childName string) string {
	return fmt.Sprintf("/v1.0/%s/%d/%s", parentName, parentID, childName)
}

// resolveChildName returns the URL segment for a child entity.
// If the entity implements ChildEntity, its ChildEntityName is used;
// otherwise it falls back to EntityName.
func resolveChildName[C Entity](c C) string {
	if ce, ok := any(c).(ChildEntity); ok {
		return ce.ChildEntityName()
	}
	return c.EntityName()
}

// resolveChildParentName returns the URL segment that names the parent
// collection in a child URL. If the child implements ChildParentEntity, its
// ChildParentName is used; otherwise it falls back to P's EntityName.
func resolveChildParentName[P Entity, C Entity](c C) string {
	if cp, ok := any(c).(ChildParentEntity); ok {
		return cp.ChildParentName()
	}
	var parent P
	return parent.EntityName()
}

// typedChildPath builds the collection URL for the typed child calls, so that
// GetChild, ListChild, CreateChild, UpdateChild and DeleteChild resolve both
// segments the same way.
func typedChildPath[P Entity, C Entity](parentID int64, c C) string {
	return childPath(resolveChildParentName[P](c), parentID, resolveChildName(c))
}

// Deprecated: Use ListChild which provides automatic pagination.
// GetChild fetches child entities for a parent entity (first page only). A
// response without an items member returns *MissingItemsError. The URL
// resolves {parent} and {child} as for UpdateChild.
func GetChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64) ([]*C, error) {
	var child C
	path := typedChildPath[P](parentID, child)
	items, _, err := fetchPage[json.RawMessage](ctx, c, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var result []*C
	for _, raw := range items {
		var entity C
		if err := json.Unmarshal(raw, &entity); err != nil {
			return nil, fmt.Errorf("autotask: decoding %s child: %w", child.EntityName(), err)
		}
		result = append(result, &entity)
	}
	return result, nil
}

// ListChild fetches all child entities for a parent, with automatic pagination.
func ListChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64) ([]*C, error) {
	var allItems []*C
	for entity, err := range ListChildIter[P, C](ctx, c, parentID) {
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, entity)
	}
	return allItems, nil
}

// ListChildIter returns an iterator over child entities with lazy pagination.
func ListChildIter[P Entity, C Entity](ctx context.Context, c *Client, parentID int64) iter.Seq2[*C, error] {
	return func(yield func(*C, error) bool) {
		var zeroC C
		path := typedChildPath[P](parentID, zeroC)
		label := zeroC.EntityName() + " child"
		pages := 0
		for {
			pages++
			if pages > maxPages {
				yield(nil, &MaxPagesExceededError{EntityName: zeroC.EntityName(), MaxPages: maxPages})
				return
			}
			nextPath, shouldContinue := fetchAndYieldTypedPage(ctx, c, http.MethodGet, path, nil, label, yield)
			if !shouldContinue || nextPath == "" {
				return
			}
			path = nextPath
		}
	}
}

// ListChildRawIter returns an iterator over child entities (untyped) for a
// parent with lazy pagination. Ranging stops fetching further pages as soon as
// the caller breaks, so a caller that needs only the first few items does not
// pull every page up to maxPages (including large blob fields such as an
// attachment's data). It is the untyped counterpart of ListChildIter. When the
// entity types are known, ListChildRawIterOf resolves both URL segments from
// them instead of taking them as strings.
func ListChildRawIter(ctx context.Context, c *Client, parentEntityName string, parentID int64, childEntityName string) iter.Seq2[map[string]any, error] {
	return listChildRawPath(ctx, c, childPath(parentEntityName, parentID, childEntityName), childEntityName)
}

// ListChildRawIterOf is ListChildRawIter with the URL built from the entity
// types, as for ListChildIter: the parent segment is C's ChildParentName or
// P's EntityName, and the child segment is C's ChildEntityName or EntityName.
// Items are still yielded as untyped maps, for a caller that needs the raw
// fields (for example to rewrite a field before decoding).
func ListChildRawIterOf[P Entity, C Entity](ctx context.Context, c *Client, parentID int64) iter.Seq2[map[string]any, error] {
	var child C
	return listChildRawPath(ctx, c, typedChildPath[P](parentID, child), child.EntityName())
}

// listChildRawPath pages through the untyped child collection starting at
// firstPath; each range starts again at firstPath. maxPagesName names the
// collection in a MaxPagesExceededError.
func listChildRawPath(ctx context.Context, c *Client, firstPath, maxPagesName string) iter.Seq2[map[string]any, error] {
	return func(yield func(map[string]any, error) bool) {
		path := firstPath
		pages := 0
		for {
			pages++
			if pages > maxPages {
				yield(nil, &MaxPagesExceededError{EntityName: maxPagesName, MaxPages: maxPages})
				return
			}
			nextPath, shouldContinue := fetchAndYieldRawPage(ctx, c, http.MethodGet, path, nil, yield)
			if !shouldContinue || nextPath == "" {
				return
			}
			path = nextPath
		}
	}
}

// ListChildRaw fetches all child entities (untyped) for a parent, with automatic
// pagination. For large or blob-heavy collections, prefer ListChildRawIter to
// stop early instead of buffering every page.
func ListChildRaw(ctx context.Context, c *Client, parentEntityName string, parentID int64, childEntityName string) ([]map[string]any, error) {
	var allItems []map[string]any
	for item, err := range ListChildRawIter(ctx, c, parentEntityName, parentID, childEntityName) {
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, item)
	}
	return allItems, nil
}

// CreateChildRaw creates a child entity (untyped) under a parent.
func CreateChildRaw(ctx context.Context, c *Client, parentEntityName string, parentID int64, childEntityName string, data map[string]any) (map[string]any, error) {
	path := childPath(parentEntityName, parentID, childEntityName)
	var resp map[string]any
	if err := c.do(ctx, http.MethodPost, path, data, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateChild creates a child entity under a parent and, when the response
// carries an itemId, sets the server-assigned id in child (see EntityWithID),
// so create each record from a new value. As with Create, a nil error does
// not guarantee the id is set: check ID before relying on it, and do not
// repeat the create because it is unset. The URL resolves {parent} and {child}
// as for UpdateChild.
func CreateChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64, child *C) (*C, error) {
	if child == nil {
		return nil, fmt.Errorf("autotask: child entity must not be nil")
	}
	path := typedChildPath[P](parentID, *child)
	var resp struct {
		ItemID *int64 `json:"itemId"`
	}
	if err := c.do(ctx, http.MethodPost, path, child, &resp); err != nil {
		return nil, err
	}
	if resp.ItemID != nil {
		assignCreatedID(child, *resp.ItemID)
	}
	return child, nil
}

// UpdateChildRaw updates a child entity (untyped) through its parent:
// PATCH /v1.0/{parent}/{parentID}/{child}. data carries the child's id and the
// fields to change. It returns the decoded response body.
func UpdateChildRaw(ctx context.Context, c *Client, parentEntityName string, parentID int64, childEntityName string, data map[string]any) (map[string]any, error) {
	path := childPath(parentEntityName, parentID, childEntityName)
	var resp map[string]any
	if err := c.do(ctx, http.MethodPatch, path, data, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// UpdateChild updates a child entity through its parent:
// PATCH /v1.0/{parent}/{parentID}/{child}. Use it for entities whose update the
// API offers only on the parent's child collection; Update sends the root URL
// instead. Like Update it sends every field that is set, and the child's id
// travels in the body, so child must have its ID set. The result follows
// Update: the entity the API returns when the response carries one, else child.
// {parent} is P's EntityName unless C implements ChildParentEntity, and
// {child} is C's ChildEntityName when it has one, else its EntityName.
func UpdateChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64, child *C) (*C, error) {
	if child == nil {
		return nil, fmt.Errorf("autotask: child entity must not be nil")
	}
	return patchEntity(ctx, c, typedChildPath[P](parentID, *child), child)
}

// DeleteChildRaw deletes a child entity through its parent:
// DELETE /v1.0/{parent}/{parentID}/{child}/{childID}.
func DeleteChildRaw(ctx context.Context, c *Client, parentEntityName string, parentID int64, childEntityName string, childID int64) error {
	path := fmt.Sprintf("%s/%d", childPath(parentEntityName, parentID, childEntityName), childID)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// DeleteChild deletes a child entity through its parent:
// DELETE /v1.0/{parent}/{parentID}/{child}/{childID}. Use it for entities the
// API deletes only through the parent's URL; Delete sends the root URL
// /v1.0/{entity}/{id} instead. {parent} and {child} resolve as for UpdateChild.
func DeleteChild[P Entity, C Entity](ctx context.Context, c *Client, parentID, childID int64) error {
	var child C
	path := fmt.Sprintf("%s/%d", typedChildPath[P](parentID, child), childID)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
