package orders

import (
	"context"
	"database/sql"
	"fmt"
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
	if batchSize < 1 {
		return 0, fmt.Errorf("%w: batch size %d", ErrInvalidInput, batchSize)
	}
	committed := 0
	batch := make([]Product, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := WithTx(ctx, db, func(tx *sql.Tx) error {
			repo := NewProductRepo(tx)
			for _, p := range batch {
				if err := repo.Upsert(ctx, p); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		committed += len(batch)
		batch = batch[:0]
		return nil
	}
	for p := range products {
		batch = append(batch, p)
		if len(batch) == batchSize {
			if err := flush(); err != nil {
				return committed, err
			}
		}
	}
	if err := flush(); err != nil {
		return committed, err
	}
	return committed, nil
}
