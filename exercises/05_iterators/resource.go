package iterators

import (
	"errors"
	"io"
	"iter"
)

// ErrBatchSize is yielded by ReadBatches when size < 1. The source is never
// opened in that case.
var ErrBatchSize = errors.New("batch size must be positive")

// ReadBatches yields batches of up to size lines read from a source obtained
// from open. Lines are returned without their terminator.
//
// Resource rules, which are the point of this exercise:
//   - open is not called until the first advance of the returned sequence.
//   - The source is closed exactly once when iteration finishes normally, fails,
//     or is stopped by a consumer break. A consumer break cannot receive an
//     error, so a Close error during a break is dropped.
//   - If open fails, (nil, "open source: %w") is yielded and iteration ends.
//   - If reading fails, the partial batch is discarded, (nil, err) is yielded
//     and iteration ends.
//   - If Close fails after a complete read, (nil, "close source: %w") is
//     yielded as the final item.
//   - size < 1 yields (nil, ErrBatchSize) once without calling open.
func ReadBatches(open func() (io.ReadCloser, error), size int) iter.Seq2[[]string, error] {
	return func(yield func([]string, error) bool) {}
}
