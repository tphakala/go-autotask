package autotask

import "fmt"

// maxPages is the safety limit on pagination loops to prevent infinite loops
// from malformed API responses that cycle nextPageUrl values.
const maxPages = 1000

// ErrMaxPagesExceeded is returned when a pagination loop exceeds maxPages.
type ErrMaxPagesExceeded struct {
	EntityName string
	MaxPages   int
}

func (e *ErrMaxPagesExceeded) Error() string {
	return fmt.Sprintf("autotask: exceeded maximum page limit (%d) fetching %s", e.MaxPages, e.EntityName)
}
