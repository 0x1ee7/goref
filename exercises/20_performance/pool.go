package perf

import (
	"errors"
	"io"
)

// ErrBadName reports a record whose name contains a tab, CR or LF, which the
// line format cannot represent.
var ErrBadName = errors.New("perf: name contains tab or newline")

// Record is one row of the tab-separated format written by WriteRecords.
type Record struct {
	ID   int64
	Name string
}

// WriteRecords writes one "<ID>\t<Name>\n" line per record to w and returns the
// number of bytes written. The whole batch is encoded into a scratch buffer
// borrowed from a sync.Pool and written with a single Write call. If any name is
// invalid, nothing is written and ErrBadName is returned. An empty batch writes
// nothing and makes no Write call.
func WriteRecords(w io.Writer, recs []Record) (int64, error) {
	return 0, nil
}
