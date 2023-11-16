package misc

import "fmt"

// ErrAtPage is an error with a page number.
type ErrAtPage struct {
	Page int
	Err  error
}

// Error implements error interface.
func (e ErrAtPage) Error() string {
	return fmt.Sprintf("at page %d: %v", e.Page, e.Err)
}

// Unwrap implements error interface.
func (e ErrAtPage) Unwrap() error {
	return e.Err
}

// ListAll lists objects by batches.
func ListAll[T any](startPage int, listFn func(page int) ([]T, error)) ([]T, error) {
	var (
		result []T
		err    error
		page   = startPage
	)

	for {
		var nodes []T
		if nodes, err = listFn(page); err != nil {
			return nil, ErrAtPage{Page: page, Err: err}
		}

		if len(nodes) == 0 {
			break
		}

		result = append(result, nodes...)
		page++
	}

	return result, nil
}
