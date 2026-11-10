package backup_test

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	backup "goref/exercises/12_files_and_encoding"
)

func gz(t *testing.T, data string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := backup.Compress(&buf, strings.NewReader(data), gzip.BestSpeed); err != nil {
		t.Fatalf("setup Compress: %v", err)
	}
	return buf.Bytes()
}

func TestPart7GzipRoundTrip(t *testing.T) {
	t.Run("compress then decompress returns the input and counts", func(t *testing.T) {
		data := strings.Repeat("backup line with some repetition\n", 50_000)
		var packed, out bytes.Buffer
		n, err := backup.Compress(&packed, strings.NewReader(data), gzip.DefaultCompression)
		if err != nil {
			t.Fatalf("Compress: %v", err)
		}
		if n != int64(len(data)) {
			t.Errorf("Compress returned %d, want %d", n, len(data))
		}
		if packed.Len() >= len(data)/10 {
			t.Errorf("compressed size %d not much smaller than %d", packed.Len(), len(data))
		}
		m, err := backup.Decompress(&out, &packed)
		if err != nil {
			t.Fatalf("Decompress: %v", err)
		}
		if m != int64(len(data)) || out.String() != data {
			t.Errorf("Decompress wrote %d bytes; content match = %v", m, out.String() == data)
		}
	})

	t.Run("identical input gives identical bytes (no name, zero mtime)", func(t *testing.T) {
		a := gz(t, "same content")
		b := gz(t, "same content")
		if !bytes.Equal(a, b) {
			t.Errorf("compressed outputs differ:\n%x\n%x", a, b)
		}
	})

	t.Run("empty input compresses and round trips", func(t *testing.T) {
		var out bytes.Buffer
		n, err := backup.Decompress(&out, bytes.NewReader(gz(t, "")))
		if err != nil || n != 0 || out.Len() != 0 {
			t.Errorf("Decompress(empty gzip) = %d, %v, %d bytes", n, err, out.Len())
		}
	})

	t.Run("concatenated members decode as one stream", func(t *testing.T) {
		in := append(gz(t, "first,"), gz(t, "second")...)
		var out bytes.Buffer
		if _, err := backup.Decompress(&out, bytes.NewReader(in)); err != nil {
			t.Fatalf("Decompress: %v", err)
		}
		if diff := cmp.Diff("first,second", out.String()); diff != "" {
			t.Errorf("output (-want +got):\n%s", diff)
		}
	})

	t.Run("invalid compression level is an error, not corruption", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := backup.Compress(&buf, strings.NewReader("x"), 42)
		if err == nil {
			t.Fatal("Compress accepted level 42")
		}
		if errors.Is(err, backup.ErrCorrupt) {
			t.Errorf("level error wraps ErrCorrupt: %v", err)
		}
		buf.Reset()
		if _, err := backup.Compress(&buf, strings.NewReader("x"), 6); err != nil {
			t.Fatalf("Compress rejected valid level 6: %v", err)
		}
	})
}

func TestPart7GzipCorruption(t *testing.T) {
	valid := gz(t, strings.Repeat("payload ", 1000))

	corrupt := map[string][]byte{
		"empty input":     {},
		"not gzip":        []byte("plain text, not gzip at all"),
		"bad checksum":    flipByte(valid, len(valid)-5), // CRC32 trailer byte
		"truncated":       valid[:len(valid)/2],
		"trailer missing": valid[:len(valid)-3],
		"garbage deflate": append(append([]byte{}, valid[:10]...), bytes.Repeat([]byte{0xff}, 40)...),
	}
	for name, in := range corrupt {
		t.Run("Decompress rejects "+name, func(t *testing.T) {
			_, err := backup.Decompress(io.Discard, bytes.NewReader(in))
			if !errors.Is(err, backup.ErrCorrupt) {
				t.Fatalf("Decompress error = %v, want ErrCorrupt", err)
			}
		})
		t.Run("VerifyGzip rejects "+name, func(t *testing.T) {
			err := backup.VerifyGzip(bytes.NewReader(in))
			if !errors.Is(err, backup.ErrCorrupt) {
				t.Fatalf("VerifyGzip error = %v, want ErrCorrupt", err)
			}
		})
	}

	t.Run("VerifyGzip accepts a valid stream", func(t *testing.T) {
		if err := backup.VerifyGzip(bytes.NewReader(valid)); err != nil {
			t.Fatalf("VerifyGzip(valid): %v", err)
		}
	})

	t.Run("destination write errors are not ErrCorrupt", func(t *testing.T) {
		boom := errors.New("disk full")
		_, err := backup.Decompress(failWriter{err: boom}, bytes.NewReader(valid))
		if !errors.Is(err, boom) {
			t.Fatalf("Decompress error = %v, want to wrap %v", err, boom)
		}
		if errors.Is(err, backup.ErrCorrupt) {
			t.Errorf("writer failure reported as corruption: %v", err)
		}
	})
}

func flipByte(b []byte, i int) []byte {
	out := append([]byte{}, b...)
	out[i] ^= 0xff
	return out
}

type failWriter struct{ err error }

func (w failWriter) Write([]byte) (int, error) { return 0, w.err }

// snapshotBytes builds a raw snapshot by hand so the reader is tested against
// the documented layout, not against WriteSnapshot.
func snapshotBytes(version uint16, flags uint16, count uint32, extra []byte, payload []byte, magic string) []byte {
	var b bytes.Buffer
	b.WriteString(magic)
	_ = binary.Write(&b, binary.BigEndian, version)
	_ = binary.Write(&b, binary.BigEndian, flags)
	_ = binary.Write(&b, binary.BigEndian, count)
	b.Write(extra)
	b.Write(payload)
	_ = binary.Write(&b, binary.BigEndian, crc32.Checksum(b.Bytes(), crc32.MakeTable(crc32.Castagnoli)))
	return b.Bytes()
}

func TestPart7Snapshot(t *testing.T) {
	createdUs := stampU.Truncate(time.Microsecond)
	usBytes := func(ts time.Time) []byte {
		var b bytes.Buffer
		_ = binary.Write(&b, binary.BigEndian, ts.UnixMicro())
		return b.Bytes()
	}

	t.Run("write then read version 2 round trip", func(t *testing.T) {
		in := backup.Snapshot{Flags: 0x0003, EntryCount: 42, CreatedAt: stampU.Add(777 * time.Nanosecond), Payload: []byte("entries...")}
		var buf bytes.Buffer
		if err := backup.WriteSnapshot(&buf, in); err != nil {
			t.Fatalf("WriteSnapshot: %v", err)
		}
		got, err := backup.ReadSnapshot(&buf)
		if err != nil {
			t.Fatalf("ReadSnapshot: %v", err)
		}
		want := backup.Snapshot{Version: 2, Flags: 3, EntryCount: 42, CreatedAt: createdUs, Payload: []byte("entries...")}
		if diff := cmp.Diff(want, got, cmp.Comparer(func(a, b time.Time) bool { return a.Equal(b) })); diff != "" {
			t.Errorf("snapshot (-want +got):\n%s", diff)
		}
	})

	t.Run("layout is exactly the documented bytes", func(t *testing.T) {
		var buf bytes.Buffer
		if err := backup.WriteSnapshot(&buf, backup.Snapshot{Flags: 1, EntryCount: 2, CreatedAt: createdUs, Payload: []byte("p")}); err != nil {
			t.Fatalf("WriteSnapshot: %v", err)
		}
		head := buf.Bytes()
		if len(head) < 20 {
			t.Fatalf("wrote %d bytes, want at least 20", len(head))
		}
		head = head[:20]
		want := snapshotBytes(2, 1, 2, usBytes(createdUs), []byte("p"), "BKSN")[:20]
		if !bytes.Equal(head, want) {
			t.Errorf("header bytes = %x, want %x", head, want)
		}
	})

	t.Run("WriteSnapshot rejects zero CreatedAt", func(t *testing.T) {
		var buf bytes.Buffer
		if err := backup.WriteSnapshot(&buf, backup.Snapshot{Payload: []byte("x")}); err == nil {
			t.Fatal("WriteSnapshot accepted a zero CreatedAt")
		}
		if err := backup.WriteSnapshot(&buf, backup.Snapshot{CreatedAt: createdUs, Payload: []byte("x")}); err != nil {
			t.Fatalf("WriteSnapshot rejected a valid CreatedAt: %v", err)
		}
	})

	t.Run("reads version 1 with zero CreatedAt", func(t *testing.T) {
		raw := snapshotBytes(1, 9, 5, nil, []byte("v1 payload"), "BKSN")
		got, err := backup.ReadSnapshot(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("ReadSnapshot(v1): %v", err)
		}
		want := backup.Snapshot{Version: 1, Flags: 9, EntryCount: 5, Payload: []byte("v1 payload")}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("v1 snapshot (-want +got):\n%s", diff)
		}
		if !got.CreatedAt.IsZero() {
			t.Errorf("CreatedAt = %v, want zero for version 1", got.CreatedAt)
		}
	})

	t.Run("empty payload round trips", func(t *testing.T) {
		var buf bytes.Buffer
		if err := backup.WriteSnapshot(&buf, backup.Snapshot{CreatedAt: createdUs}); err != nil {
			t.Fatalf("WriteSnapshot: %v", err)
		}
		if buf.Len() != 24 {
			t.Errorf("empty snapshot is %d bytes, want 24 (20 header + 4 crc)", buf.Len())
		}
		got, err := backup.ReadSnapshot(&buf)
		if err != nil || len(got.Payload) != 0 {
			t.Errorf("ReadSnapshot = %+v, %v", got, err)
		}
	})

	t.Run("large payload round trips", func(t *testing.T) {
		payload := bytes.Repeat([]byte("0123456789abcdef"), 1<<16) // 1 MiB
		var buf bytes.Buffer
		if err := backup.WriteSnapshot(&buf, backup.Snapshot{EntryCount: 1, CreatedAt: createdUs, Payload: payload}); err != nil {
			t.Fatalf("WriteSnapshot: %v", err)
		}
		got, err := backup.ReadSnapshot(&buf)
		if err != nil || !bytes.Equal(got.Payload, payload) {
			t.Fatalf("ReadSnapshot payload mismatch: err=%v len=%d", err, len(got.Payload))
		}
	})

	t.Run("a flipped payload byte is a checksum error", func(t *testing.T) {
		var buf bytes.Buffer
		if err := backup.WriteSnapshot(&buf, backup.Snapshot{CreatedAt: createdUs, Payload: []byte("integrity matters")}); err != nil {
			t.Fatalf("WriteSnapshot: %v", err)
		}
		raw := flipByte(buf.Bytes(), 22)
		_, err := backup.ReadSnapshot(bytes.NewReader(raw))
		if !errors.Is(err, backup.ErrSnapshotChecksum) {
			t.Fatalf("ReadSnapshot error = %v, want ErrSnapshotChecksum", err)
		}
	})

	t.Run("bad magic", func(t *testing.T) {
		raw := snapshotBytes(2, 0, 0, usBytes(createdUs), nil, "NOPE")
		_, err := backup.ReadSnapshot(bytes.NewReader(raw))
		if !errors.Is(err, backup.ErrSnapshotMagic) {
			t.Fatalf("ReadSnapshot error = %v, want ErrSnapshotMagic", err)
		}
	})

	t.Run("unsupported version", func(t *testing.T) {
		raw := snapshotBytes(3, 0, 0, usBytes(createdUs), nil, "BKSN")
		_, err := backup.ReadSnapshot(bytes.NewReader(raw))
		if !errors.Is(err, backup.ErrSnapshotVersion) {
			t.Fatalf("ReadSnapshot error = %v, want ErrSnapshotVersion", err)
		}
	})

	t.Run("every proper prefix is truncated", func(t *testing.T) {
		var buf bytes.Buffer
		if err := backup.WriteSnapshot(&buf, backup.Snapshot{CreatedAt: createdUs}); err != nil {
			t.Fatalf("WriteSnapshot: %v", err)
		}
		full := buf.Bytes()
		for n := 0; n < len(full); n++ {
			_, err := backup.ReadSnapshot(bytes.NewReader(full[:n]))
			if !errors.Is(err, backup.ErrSnapshotTruncated) {
				t.Errorf("prefix of %d bytes: error = %v, want ErrSnapshotTruncated", n, err)
			}
		}
	})

	t.Run("a payload cut off with a valid header is a checksum error", func(t *testing.T) {
		var buf bytes.Buffer
		if err := backup.WriteSnapshot(&buf, backup.Snapshot{CreatedAt: createdUs, Payload: []byte("0123456789")}); err != nil {
			t.Fatalf("WriteSnapshot: %v", err)
		}
		full := buf.Bytes()
		cut := append(append([]byte{}, full[:len(full)-8]...), full[len(full)-4:]...)
		_, err := backup.ReadSnapshot(bytes.NewReader(cut))
		if !errors.Is(err, backup.ErrSnapshotChecksum) {
			t.Fatalf("ReadSnapshot error = %v, want ErrSnapshotChecksum", err)
		}
	})
}
