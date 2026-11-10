package iterators

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"iter"
)

// ErrBatchSize reports a batch size below one.
var ErrBatchSize = errors.New("batch size must be at least 1")

// ReadBatches opens a source lazily on the first advance and yields its lines
// in batches of size. The source is closed exactly once, on normal end, on
// error, or when the consumer breaks. A close error after a complete read is
// yielded as a final error. A read error discards the partial batch. size < 1
// yields ErrBatchSize without opening anything.
func ReadBatches(open func() (io.ReadCloser, error), size int) iter.Seq2[[]string, error] {
	return func(yield func([]string, error) bool) {
		if size < 1 {
			yield(nil, ErrBatchSize)
			return
		}
		rc, err := open()
		if err != nil {
			yield(nil, fmt.Errorf("open source: %w", err))
			return
		}
		closed := false
		defer func() {
			if !closed {
				closed = true
				rc.Close() // nothing useful to do with this error on early exit
			}
		}()

		sc := bufio.NewScanner(rc)
		batch := make([]string, 0, size)
		for sc.Scan() {
			batch = append(batch, sc.Text())
			if len(batch) == size {
				if !yield(batch, nil) {
					return
				}
				batch = make([]string, 0, size)
			}
		}
		if err := sc.Err(); err != nil {
			yield(nil, fmt.Errorf("read source: %w", err))
			return
		}
		if len(batch) > 0 && !yield(batch, nil) {
			return
		}
		closed = true
		if err := rc.Close(); err != nil {
			yield(nil, fmt.Errorf("close source: %w", err))
		}
	}
}
