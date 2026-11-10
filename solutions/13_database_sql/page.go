package orders

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Cursor identifies a position in the order list, ordered by (PlacedAt, ID).
// It is serialized into an opaque, URL-safe token for clients.
type Cursor struct {
	PlacedAt time.Time
	ID       int64
}

// Encode returns the URL-safe, unpadded token for c. The token is opaque to
// clients; PlacedAt is written with microsecond precision.
func (c Cursor) Encode() string {
	raw := strconv.FormatInt(c.PlacedAt.UnixMicro(), 10) + ":" + strconv.FormatInt(c.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses a token produced by Encode. Empty input, bad base64, a
// missing separator, or a non-integer field returns ErrInvalidInput. Decoding
// never panics, whatever the input.
func DecodeCursor(token string) (Cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Cursor{}, fmt.Errorf("%w: cursor encoding: %w", ErrInvalidInput, err)
	}
	ts, id, ok := strings.Cut(string(raw), ":")
	if !ok {
		return Cursor{}, fmt.Errorf("%w: cursor has no separator", ErrInvalidInput)
	}
	micros, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return Cursor{}, fmt.Errorf("%w: cursor time: %w", ErrInvalidInput, err)
	}
	orderID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return Cursor{}, fmt.Errorf("%w: cursor id: %w", ErrInvalidInput, err)
	}
	return Cursor{PlacedAt: time.UnixMicro(micros).UTC(), ID: orderID}, nil
}

// OrderPage is one page of orders in (PlacedAt, ID) ascending order.
type OrderPage struct {
	Orders []Order
	// NextToken continues after the last order of this page. It is empty when
	// the page is not full, meaning the next call would return nothing.
	NextToken string
}

// PageOrders returns up to limit orders strictly after the position encoded by
// afterToken. An empty afterToken starts from the beginning. A malformed token
// or limit below 1 returns ErrInvalidInput.
//
// NextToken is set when len(Orders) == limit, even if no rows follow; the next
// call then returns an empty page with an empty NextToken. Pages do not overlap
// even when older rows are inserted between calls.
func (r *OrderRepo) PageOrders(ctx context.Context, afterToken string, limit int) (OrderPage, error) {
	if limit < 1 {
		return OrderPage{}, fmt.Errorf("%w: limit %d", ErrInvalidInput, limit)
	}
	var (
		query = `SELECT ` + orderCols + ` FROM orders ORDER BY placed_at, id LIMIT ?`
		args  = []any{limit}
	)
	if afterToken != "" {
		after, err := DecodeCursor(afterToken)
		if err != nil {
			return OrderPage{}, err
		}
		query = `SELECT ` + orderCols + ` FROM orders
			WHERE (placed_at, id) > (?, ?) ORDER BY placed_at, id LIMIT ?`
		args = []any{after.PlacedAt.UnixMicro(), after.ID, limit}
	}
	list, err := queryAll(ctx, r.db, scanOrder, query, args...)
	if err != nil {
		return OrderPage{}, err
	}
	page := OrderPage{Orders: list}
	if len(list) == limit {
		last := list[len(list)-1]
		page.NextToken = Cursor{PlacedAt: last.PlacedAt, ID: last.ID}.Encode()
	}
	return page, nil
}

// PageOrdersOffset returns up to limit orders in (PlacedAt, ID) order after
// skipping offset rows. Unlike PageOrders it shifts when rows are inserted
// before the offset. A negative offset or limit below 1 returns ErrInvalidInput.
func (r *OrderRepo) PageOrdersOffset(ctx context.Context, offset, limit int) ([]Order, error) {
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset %d", ErrInvalidInput, offset)
	}
	if limit < 1 {
		return nil, fmt.Errorf("%w: limit %d", ErrInvalidInput, limit)
	}
	return queryAll(ctx, r.db, scanOrder,
		`SELECT `+orderCols+` FROM orders ORDER BY placed_at, id LIMIT ? OFFSET ?`, limit, offset)
}
