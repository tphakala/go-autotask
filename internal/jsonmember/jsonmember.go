// Package jsonmember decodes one JSON object member and records whether the
// member was present.
package jsonmember

import "encoding/json"

// Member holds the decoded value of one JSON object member and whether the
// member was present. encoding/json calls UnmarshalJSON only when the member
// exists, null included, and not when it is absent or when the whole body is
// null (measured with go1.27.4). A plain slice cannot tell an absent member
// from an empty one; Present can.
//
// The root package tests TestListPageWithoutItemsIsMissingItemsError
// (absent member and null body) and TestListPageWithEmptyOrNullItemsIsEmpty
// ("items": null) pin that behavior for list pages, and the metadata tests
// TestMetadataResponseWithoutResultIsMissingMemberError and
// TestMetadataResponseWithPresentEmptyMemberIsAResult pin it for the metadata
// calls.
type Member[T any] struct {
	Present bool
	Value   T
}

// UnmarshalJSON marks the member present and decodes data into Value.
func (m *Member[T]) UnmarshalJSON(data []byte) error {
	m.Present = true
	return json.Unmarshal(data, &m.Value)
}
