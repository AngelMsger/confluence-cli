package apiclient

import "context"

// Search runs a CQL query and returns one page of normalized hits.
func (c *apiClient) Search(ctx context.Context, cql string, opt ListOpts) (ListResult[SearchHit], error) {
	limit := c.limitOf(opt)
	q := offsetQuery(opt.Cursor, limit)
	q.Set("cql", cql)

	var raw rawSearchList
	if err := c.getJSON(ctx, c.v1Base()+"/search", q, &raw); err != nil {
		return ListResult[SearchHit]{}, err
	}
	res := ListResult[SearchHit]{Next: nextOffsetToken(opt.Cursor, limit, len(raw.Results))}
	for _, r := range raw.Results {
		hit := SearchHit{
			ID:           r.Content.ID,
			Type:         firstNonEmpty(r.Content.Type, r.EntityType),
			Title:        firstNonEmpty(r.Content.Title, r.Title),
			Excerpt:      r.Excerpt,
			LastModified: r.LastModified,
			URL:          c.absURL(r.Content.Links.WebUI),
		}
		if r.Content.Space != nil {
			hit.SpaceKey = r.Content.Space.Key
		}
		res.Items = append(res.Items, hit)
	}
	return res, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
