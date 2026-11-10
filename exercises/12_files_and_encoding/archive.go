package backup

import (
	"errors"
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
	return 0, errors.New("not implemented")
}

// Decompress streams a gzip stream from src into dst, including concatenated
// members. It returns the number of uncompressed bytes written.
func Decompress(dst io.Writer, src io.Reader) (int64, error) {
	return 0, errors.New("not implemented")
}

// VerifyGzip decompresses src to io.Discard and reports whether it is a
// complete, valid gzip stream. Its error is nil or wraps ErrCorrupt.
func VerifyGzip(src io.Reader) error {
	return errors.New("not implemented")
}
