package backup

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
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

const (
	headerV1 = 12 // magic + version + flags + entry_count
	headerV2 = 20 // headerV1 + created_us
	trailer  = 4
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

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
	if s.CreatedAt.IsZero() {
		return errors.New("write snapshot: CreatedAt is zero")
	}
	var hdr [headerV2]byte
	copy(hdr[0:4], SnapshotMagic)
	binary.BigEndian.PutUint16(hdr[4:], SnapshotVersion)
	binary.BigEndian.PutUint16(hdr[6:], s.Flags)
	binary.BigEndian.PutUint32(hdr[8:], s.EntryCount)
	binary.BigEndian.PutUint64(hdr[12:], uint64(s.CreatedAt.UnixMicro()))

	crc := crc32.New(crcTable)
	for _, part := range [][]byte{hdr[:], s.Payload} {
		if _, err := w.Write(part); err != nil {
			return fmt.Errorf("write snapshot: %w", err)
		}
		_, _ = crc.Write(part)
	}
	var tail [trailer]byte
	binary.BigEndian.PutUint32(tail[:], crc.Sum32())
	if _, err := w.Write(tail[:]); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}

// ReadSnapshot parses a version 1 or version 2 snapshot from r, which must hold
// exactly one snapshot. Errors: ErrSnapshotTruncated for short input,
// ErrSnapshotMagic, ErrSnapshotVersion for any other version, and
// ErrSnapshotChecksum when the trailer does not match.
func ReadSnapshot(r io.Reader) (Snapshot, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot: %w", err)
	}
	if len(data) < 4 {
		return Snapshot{}, ErrSnapshotTruncated
	}
	if string(data[:4]) != SnapshotMagic {
		return Snapshot{}, ErrSnapshotMagic
	}
	if len(data) < 6 {
		return Snapshot{}, ErrSnapshotTruncated
	}
	version := binary.BigEndian.Uint16(data[4:6])
	var hdrLen int
	switch version {
	case 1:
		hdrLen = headerV1
	case 2:
		hdrLen = headerV2
	default:
		return Snapshot{}, fmt.Errorf("%w: %d", ErrSnapshotVersion, version)
	}
	if len(data) < hdrLen+trailer {
		return Snapshot{}, ErrSnapshotTruncated
	}

	body := data[:len(data)-trailer]
	if crc32.Checksum(body, crcTable) != binary.BigEndian.Uint32(data[len(data)-trailer:]) {
		return Snapshot{}, ErrSnapshotChecksum
	}

	s := Snapshot{
		Version:    version,
		Flags:      binary.BigEndian.Uint16(data[6:8]),
		EntryCount: binary.BigEndian.Uint32(data[8:12]),
		Payload:    body[hdrLen:],
	}
	if version == 2 {
		s.CreatedAt = time.UnixMicro(int64(binary.BigEndian.Uint64(data[12:20]))).UTC()
	}
	return s, nil
}
