package orders

import (
	"context"
	"database/sql"
	"iter"
)

// ImportProducts upserts every product yielded by products and returns the
// number of rows committed.
//
// Rows are written in batches of batchSize, each batch in its own transaction
// (see WithTx). A batch size below 1 returns ErrInvalidInput before any write.
// products is consumed lazily: memory must not grow with the number of rows.
// Rows are upserts, so re-importing updates existing SKUs.
//
// If a row is invalid (for example a negative price) the batch containing it is
// rolled back, iteration stops, and the error is returned together with the
// number of rows committed by earlier batches. Those batches stay committed.
func ImportProducts(ctx context.Context, db *sql.DB, products iter.Seq[Product], batchSize int) (int, error) {
	return 0, errNotImplemented
}
