package backup

import (
	"errors"
	"io"
	"time"
)

// Snapshot file layout. All integers are big-endian.
//
//	version 1: magic[4] "BKSN" | version u16 = 1 | flags u16 | entry_count u32
//	version 2: magic[4] "BKSN" | version u16 = 2 | flags u16 | entry_count u32 | created_us i64
//	then:      payload (all bytes up to the trailer)
//	trailer:   crc32c u32 over every byte before it (Castagnoli polynomial)
const (
	SnapshotMagic   = "BKSN"
	SnapshotVersion = 2
)

var (
	ErrSnapshotTruncated = errors.New("snapshot truncated")
	ErrSnapshotMagic     = errors.New("snapshot bad magic")
	ErrSnapshotVersion   = errors.New("unsupported snapshot version")
	ErrSnapshotChecksum  = errors.New("snapshot checksum mismatch")
)

// Snapshot is a versioned binary header plus payload.
type Snapshot struct {
	Version    uint16 // set by ReadSnapshot; ignored by WriteSnapshot, which always writes version 2
	Flags      uint16
	EntryCount uint32
	CreatedAt  time.Time // UTC, microsecond precision; zero for version 1
	Payload    []byte
}

// WriteSnapshot writes s in the version 2 layout. CreatedAt must not be the
// zero time; it is stored as Unix microseconds (truncated).
func WriteSnapshot(w io.Writer, s Snapshot) error {
	return errors.New("not implemented")
}

// ReadSnapshot parses a version 1 or version 2 snapshot from r, which must hold
// exactly one snapshot. Errors: ErrSnapshotTruncated for short input,
// ErrSnapshotMagic, ErrSnapshotVersion for any other version, and
// ErrSnapshotChecksum when the trailer does not match.
func ReadSnapshot(r io.Reader) (Snapshot, error) {
	return Snapshot{}, errors.New("not implemented")
}
