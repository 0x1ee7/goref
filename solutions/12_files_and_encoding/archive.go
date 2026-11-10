package backup

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
)

// ErrCorrupt is wrapped by Decompress and VerifyGzip for input that is not a
// valid gzip stream: empty input, a bad header, a checksum mismatch, truncation
// or invalid deflate data. Errors from the destination writer are not wrapped
// with ErrCorrupt.
var ErrCorrupt = errors.New("corrupt gzip stream")

// Compress streams src into dst as gzip at the given level (any value accepted
// by gzip.NewWriterLevel). The header carries no name and a zero mtime, so
// identical content always produces identical bytes. It returns the number of
// uncompressed bytes read from src.
func Compress(dst io.Writer, src io.Reader, level int) (int64, error) {
	zw, err := gzip.NewWriterLevel(dst, level)
	if err != nil {
		return 0, fmt.Errorf("compress: %w", err)
	}
	n, err := io.Copy(zw, src)
	if err != nil {
		_ = zw.Close()
		return n, fmt.Errorf("compress: %w", err)
	}
	if err := zw.Close(); err != nil {
		return n, fmt.Errorf("compress: %w", err)
	}
	return n, nil
}

// Decompress streams a gzip stream from src into dst, including concatenated
// members. It returns the number of uncompressed bytes written.
func Decompress(dst io.Writer, src io.Reader) (int64, error) {
	sr := &recordingReader{r: src}
	dw := &recordingWriter{w: dst}

	zr, err := gzip.NewReader(sr)
	if err != nil {
		return 0, classify(sr, dw, err)
	}
	defer zr.Close()
	n, err := io.Copy(dw, zr)
	if err != nil {
		return n, classify(sr, dw, err)
	}
	return n, nil
}

// VerifyGzip decompresses src to io.Discard and reports whether it is a
// complete, valid gzip stream. Its error is nil or wraps ErrCorrupt.
func VerifyGzip(src io.Reader) error {
	_, err := Decompress(io.Discard, src)
	return err
}

// classify decides which side of the pipe failed. Destination errors and
// source I/O errors are returned as-is; anything else is a malformed stream.
func classify(sr *recordingReader, dw *recordingWriter, err error) error {
	switch {
	case dw.err != nil:
		return fmt.Errorf("decompress: write: %w", dw.err)
	case sr.err != nil:
		return fmt.Errorf("decompress: read: %w", sr.err)
	default:
		return fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
}

// recordingReader remembers the first non-EOF error from the source so that
// gzip failures can be told apart from source failures.
type recordingReader struct {
	r   io.Reader
	err error
}

func (r *recordingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		r.err = err
	}
	return n, err
}

// recordingWriter remembers the first error returned by the destination.
type recordingWriter struct {
	w   io.Writer
	err error
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if err != nil {
		w.err = err
	}
	return n, err
}
