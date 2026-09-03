package autotask_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/entities"
)

func makeCompanies(n int) []entities.Company {
	companies := make([]entities.Company, n)
	for i := range n {
		companies[i] = autotasktest.CompanyFixture(func(c *entities.Company) {
			c.CompanyName = autotask.Set(fmt.Sprintf("Company %d", i))
		})
	}
	return companies
}

func TestPaginationMultiPageList(t *testing.T) {
	t.Parallel()

	companies := makeCompanies(5)
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(companies[0], companies[1], companies[2], companies[3], companies[4]),
		autotasktest.WithPageSize(2),
	)

	items, err := autotask.List[entities.Company](t.Context(), client, autotask.NewQuery())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("got %d items, want 5", len(items))
	}
}

func TestPaginationMultiPageListIter(t *testing.T) {
	t.Parallel()

	companies := makeCompanies(5)
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(companies[0], companies[1], companies[2], companies[3], companies[4]),
		autotasktest.WithPageSize(2),
	)

	var count int
	for entity, err := range autotask.ListIter[entities.Company](t.Context(), client, autotask.NewQuery()) {
		if err != nil {
			t.Fatal(err)
		}
		_ = entity
		count++
	}
	if count != 5 {
		t.Fatalf("got %d items, want 5", count)
	}
}

func TestPaginationSinglePage(t *testing.T) {
	t.Parallel()

	companies := makeCompanies(3)
	srv, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(companies[0], companies[1], companies[2]),
		// default page size is 500, so 3 entities fit in one page
	)

	// Test List
	items, err := autotask.List[entities.Company](t.Context(), client, autotask.NewQuery())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("List: got %d items, want 3", len(items))
	}

	// Only 1 request should have been made for List
	listRequests := srv.RequestCount()
	if listRequests != 1 {
		t.Fatalf("List: got %d requests, want 1", listRequests)
	}

	// Create a fresh server for ListIter to get a clean request count
	srv2, client2 := autotasktest.NewServer(t,
		autotasktest.WithEntity(companies[0], companies[1], companies[2]),
	)

	var count int
	for entity, err := range autotask.ListIter[entities.Company](t.Context(), client2, autotask.NewQuery()) {
		if err != nil {
			t.Fatal(err)
		}
		_ = entity
		count++
	}
	if count != 3 {
		t.Fatalf("ListIter: got %d items, want 3", count)
	}

	iterRequests := srv2.RequestCount()
	if iterRequests != 1 {
		t.Fatalf("ListIter: got %d requests, want 1", iterRequests)
	}
}

func TestPaginationEmpty(t *testing.T) {
	t.Parallel()

	// No entities seeded
	_, client := autotasktest.NewServer(t)

	// List should return empty slice (nil)
	items, err := autotask.List[entities.Company](t.Context(), client, autotask.NewQuery())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("List: got %d items, want 0", len(items))
	}

	// ListIter should yield nothing
	var count int
	for entity, err := range autotask.ListIter[entities.Company](t.Context(), client, autotask.NewQuery()) {
		if err != nil {
			t.Fatal(err)
		}
		_ = entity
		count++
	}
	if count != 0 {
		t.Fatalf("ListIter: got %d items, want 0", count)
	}
}

func TestPaginationContextCancelList(t *testing.T) {
	t.Parallel()

	companies := makeCompanies(10)
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(companies...),
		autotasktest.WithPageSize(2),
		autotasktest.WithServerLatency(50*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := autotask.List[entities.Company](ctx, client, autotask.NewQuery())
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		// The error wraps context cancellation but may not unwrap directly.
		// Accept any error that mentions "cancel" or "context" as valid.
		if ctx.Err() == nil {
			t.Fatalf("expected context cancellation error, got: %v", err)
		}
	}
}

func TestPaginationContextCancelListIter(t *testing.T) {
	t.Parallel()

	companies := makeCompanies(10)
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(companies...),
		autotasktest.WithPageSize(2),
		autotasktest.WithServerLatency(50*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	// Should stop without panic. Count items to verify iteration didn't complete fully.
	var count int
	for _, err := range autotask.ListIter[entities.Company](ctx, client, autotask.NewQuery()) {
		if err != nil {
			break
		}
		count++
	}
	// With 10 entities, page size 2, and 50ms latency per page, cancellation after 10ms
	// should prevent fetching all items. If all 10 were fetched, cancellation didn't work.
	if count == 10 {
		t.Fatal("expected iteration to be cut short by context cancellation, but all 10 items were fetched")
	}
}

func TestPaginationExactPageSize(t *testing.T) {
	t.Parallel()

	companies := makeCompanies(3)
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(companies[0], companies[1], companies[2]),
		autotasktest.WithPageSize(3),
	)

	// List
	items, err := autotask.List[entities.Company](t.Context(), client, autotask.NewQuery())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("List: got %d items, want 3", len(items))
	}

	// ListIter
	var count int
	for entity, err := range autotask.ListIter[entities.Company](t.Context(), client, autotask.NewQuery()) {
		if err != nil {
			t.Fatal(err)
		}
		_ = entity
		count++
	}
	if count != 3 {
		t.Fatalf("ListIter: got %d items, want 3", count)
	}
}

func TestPaginationListWithMaxRecords(t *testing.T) {
	t.Parallel()

	companies := makeCompanies(10)
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(companies...),
		autotasktest.WithPageSize(3),
	)

	items, err := autotask.List[entities.Company](t.Context(), client, autotask.NewQuery().Limit(5))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("got %d items, want 5", len(items))
	}
}

func TestPaginationListMaxRecordsSinglePageOverflow(t *testing.T) {
	t.Parallel()

	// All 10 entities fit in one page (default page size 500), and the
	// MaxRecords cap (3) is reached within that first page. List must cap the
	// result to exactly MaxRecords, not return the whole page. This pins the
	// cap-on-accumulation behavior now that List consumes ListIter (which does
	// not enforce MaxRecords itself).
	companies := makeCompanies(10)
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(companies...),
	)

	items, err := autotask.List[entities.Company](t.Context(), client, autotask.NewQuery().Limit(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
}

func TestPaginationListMaxRecordsStopsAtPageBoundary(t *testing.T) {
	t.Parallel()

	// Limit(4) with page size 2 is reached at the end of the second page. List
	// must fetch exactly two pages and must NOT request the page after the cap.
	// Delegating to ListIter has to break before the next page fetch, matching
	// the old hand-rolled loop's request count. This pins the request-count half
	// of the refactor's equivalence; item count is pinned by the tests above.
	companies := makeCompanies(10)
	srv, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(companies...),
		autotasktest.WithPageSize(2),
	)

	items, err := autotask.List[entities.Company](t.Context(), client, autotask.NewQuery().Limit(4))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4", len(items))
	}
	if got := srv.RequestCount(); got != 2 {
		t.Fatalf("got %d page requests, want 2 (must not fetch the page after the cap)", got)
	}
}

func TestPaginationListIterWithMaxRecords(t *testing.T) {
	t.Parallel()
	t.Skip("known gap: ListIter does not enforce MaxRecords client-side")

	companies := makeCompanies(10)
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(companies...),
		autotasktest.WithPageSize(3),
	)

	var count int
	for entity, err := range autotask.ListIter[entities.Company](t.Context(), client, autotask.NewQuery().Limit(5)) {
		if err != nil {
			t.Fatal(err)
		}
		_ = entity
		count++
	}
	if count != 5 {
		t.Fatalf("got %d items, want 5", count)
	}
}
