package perf

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
)

// ErrBadName reports a record whose name contains a tab, CR or LF, which the
// line format cannot represent.
var ErrBadName = errors.New("perf: name contains tab or newline")

// Record is one row of the tab-separated format written by WriteRecords.
type Record struct {
	ID   int64
	Name string
}

// maxPooledBuf caps the size of buffers kept in bufPool. One huge batch must not
// pin megabytes of memory in the pool for the lifetime of the process.
const maxPooledBuf = 64 << 10

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// WriteRecords writes one "<ID>\t<Name>\n" line per record to w and returns the
// number of bytes written. The whole batch is encoded into a scratch buffer
// borrowed from a sync.Pool and written with a single Write call. If any name is
// invalid, nothing is written and ErrBadName is returned. An empty batch writes
// nothing and makes no Write call.
//
// Validation happens before any byte is encoded, so a bad record cannot leave a
// partial batch behind.
func WriteRecords(w io.Writer, recs []Record) (int64, error) {
	if len(recs) == 0 {
		return 0, nil
	}
	for _, r := range recs {
		if strings.ContainsAny(r.Name, "\t\r\n") {
			return 0, ErrBadName
		}
	}

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		if buf.Cap() <= maxPooledBuf {
			bufPool.Put(buf)
		}
	}()

	for _, r := range recs {
		// AvailableBuffer lets the record be appended in place, with no
		// intermediate string or []byte from strconv.
		b := buf.AvailableBuffer()
		b = strconv.AppendInt(b, r.ID, 10)
		b = append(b, '\t')
		b = append(b, r.Name...)
		b = append(b, '\n')
		buf.Write(b)
	}

	n, err := w.Write(buf.Bytes())
	return int64(n), err
}
