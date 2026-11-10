package httpsvc

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

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
	q := ListQuery{
		Limit:  DefaultLimit,
		Cursor: v.Get("cursor"),
		Search: strings.TrimSpace(v.Get("q")),
	}
	errs := FieldErrors{}

	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > MaxLimit {
			errs["limit"] = fmt.Sprintf("must be an integer from 1 to %d", MaxLimit)
		} else {
			q.Limit = n
		}
	}
	if s := v.Get("min_qty"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			errs["min_qty"] = "must be an integer >= 0"
		} else {
			q.MinQuantity = n
		}
	}
	if q.Cursor != "" {
		if _, err := decodeCursor(q.Cursor); err != nil {
			errs["cursor"] = "is not a valid cursor"
		}
	}
	if len(errs) > 0 {
		return ListQuery{}, errs
	}
	return q, nil
}

// Values encodes q as URL query parameters. It is the inverse of
// ParseListQuery for valid queries.
func (q ListQuery) Values() url.Values {
	v := url.Values{}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Cursor != "" {
		v.Set("cursor", q.Cursor)
	}
	if q.Search != "" {
		v.Set("q", q.Search)
	}
	if q.MinQuantity > 0 {
		v.Set("min_qty", strconv.Itoa(q.MinQuantity))
	}
	return v
}

// encodeCursor makes an opaque cursor naming the last sequence returned.
func encodeCursor(seq int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte("seq:" + strconv.FormatInt(seq, 10)))
}

var errBadCursor = errors.New("bad cursor")

// decodeCursor returns the sequence number stored in cursor.
func decodeCursor(cursor string) (int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, errBadCursor
	}
	num, ok := strings.CutPrefix(string(raw), "seq:")
	if !ok {
		return 0, errBadCursor
	}
	seq, err := strconv.ParseInt(num, 10, 64)
	if err != nil || seq < 0 {
		return 0, errBadCursor
	}
	return seq, nil
}

// page returns up to q.Limit records created after sequence after that match
// the filters, and whether more matching records exist beyond them.
func (inv *inventory) page(after int64, q ListQuery) ([]record, bool) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	needle := strings.ToLower(q.Search)
	out := make([]record, 0, q.Limit)
	for _, r := range inv.order {
		if r.seq <= after {
			continue
		}
		if q.MinQuantity > 0 && r.item.Quantity < q.MinQuantity {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(r.item.Name), needle) {
			continue
		}
		if len(out) == q.Limit {
			return out, true
		}
		out = append(out, r)
	}
	return out, false
}

// handleList serves GET /items with filtering and keyset pagination.
func (inv *inventory) handleList(w http.ResponseWriter, r *http.Request) {
	q, err := ParseListQuery(r.URL.Query())
	if err != nil {
		var fe FieldErrors
		if errors.As(err, &fe) {
			writeFieldProblem(w, http.StatusBadRequest, fe)
			return
		}
		writeProblem(w, http.StatusBadRequest, err.Error())
		return
	}
	var after int64
	if q.Cursor != "" {
		after, _ = decodeCursor(q.Cursor) // validated by ParseListQuery
	}
	recs, more := inv.page(after, q)
	page := ItemPage{Items: make([]Item, 0, len(recs))}
	for _, rec := range recs {
		page.Items = append(page.Items, rec.item)
	}
	if more {
		page.NextCursor = encodeCursor(recs[len(recs)-1].seq)
	}
	writeJSON(w, http.StatusOK, page)
}
