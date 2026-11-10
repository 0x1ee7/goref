package orders

import (
	"context"
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
	return ""
}

// DecodeCursor parses a token produced by Encode. Empty input, bad base64, a
// missing separator, or a non-integer field returns ErrInvalidInput. Decoding
// never panics, whatever the input.
func DecodeCursor(token string) (Cursor, error) {
	return Cursor{}, errNotImplemented
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
	return OrderPage{}, errNotImplemented
}

// PageOrdersOffset returns up to limit orders in (PlacedAt, ID) order after
// skipping offset rows. Unlike PageOrders it shifts when rows are inserted
// before the offset. A negative offset or limit below 1 returns ErrInvalidInput.
func (r *OrderRepo) PageOrdersOffset(ctx context.Context, offset, limit int) ([]Order, error) {
	return nil, errNotImplemented
}
