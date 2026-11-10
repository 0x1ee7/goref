package httpsvc

import "net/url"

// Page size limits for GET /items.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// ItemPage is one page of items. NextCursor is empty on the last page.
type ItemPage struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// ListQuery describes a filtered, paginated item listing.
type ListQuery struct {
	Limit       int
	Cursor      string
	Search      string
	MinQuantity int
}

// ParseListQuery parses and validates limit, cursor, q and min_qty. Invalid
// values produce a FieldErrors value with one entry per offending key.
func ParseListQuery(v url.Values) (ListQuery, error) {
	return ListQuery{}, errNotImplemented
}

// Values encodes q as URL query parameters. It is the inverse of
// ParseListQuery for valid queries.
func (q ListQuery) Values() url.Values {
	return nil
}
