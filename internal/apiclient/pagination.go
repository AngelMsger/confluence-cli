package apiclient

// pagination.go provides a uniform way to consume paginated listings. It hides
// the offset-vs-cursor distinction: callers always follow the opaque
// ListResult.Next token.

// FetchPage fetches one page of results given a continuation cursor. An empty
// cursor requests the first page.
type FetchPage[T any] func(cursor string) (ListResult[T], error)

// CollectAll walks every page by following Next until the listing is exhausted
// or max items have been gathered. A max of 0 means no limit.
func CollectAll[T any](fetch FetchPage[T], max int) ([]T, error) {
	var all []T
	cursor := ""
	for {
		page, err := fetch(cursor)
		if err != nil {
			return all, err
		}
		all = append(all, page.Items...)
		if max > 0 && len(all) >= max {
			return all[:max], nil
		}
		if page.Next == "" {
			return all, nil
		}
		cursor = page.Next
	}
}
