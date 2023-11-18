// Package misc provides miscellaneous functions and types.
package misc

// Sort specifies parameters for ordering.
type Sort struct {
	By    SortBy
	Order SortOrder
}

// SortBy specifies a field to sort by.
type SortBy string

const (
	// SortByCreatedAt sorts by created at.
	SortByCreatedAt SortBy = "created_at"
	// SortByTitle sorts by title.
	SortByTitle SortBy = "title"
	// SortByUpdatedAt sorts by updated at.
	SortByUpdatedAt SortBy = "updated_at"
)

// SortOrder specifies a sort order.
type SortOrder string

const (
	// SortOrderAsc sorts in ascending order.
	SortOrderAsc SortOrder = "asc"
	// SortOrderDesc sorts in descending order.
	SortOrderDesc SortOrder = "desc"
)

// Pagination specifies pagination parameters.
type Pagination struct {
	PerPage int
	Page    int
}

// Filter is a filter for a list of items.
type Filter[T any] struct {
	Include []T
	Exclude []T
}

// PtrTernary returns ifTrue if cond is true, ifFalse if cond is false, and empty value otherwise.
func PtrTernary[T any](cond *bool, ifTrue, ifFalse T) T {
	if cond == nil {
		return *new(T)
	}
	if *cond {
		return ifTrue
	}
	return ifFalse
}

// Map applies f to each element of s and returns the result.
func Map[T, R any](s []T, f func(T) R) []R {
	r := make([]R, len(s))
	for i, v := range s {
		r[i] = f(v)
	}
	return r
}
