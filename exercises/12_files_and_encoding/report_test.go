package backup_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	backup "goref/exercises/12_files_and_encoding"
)

func reportManifest() backup.Manifest {
	return backup.Manifest{
		"b/z.txt":        {Size: 1234567, ModTime: stampU, Digest: sum("z")},
		"a,quoted \"x\"": {Size: 0, ModTime: stampU.Add(time.Nanosecond), Digest: sum("")},
		"a/first.bin":    {Size: 7, ModTime: time.Date(2026, 3, 4, 5, 6, 7, 123456000, time.FixedZone("CET", 3600)), Digest: sum("f")},
	}
}

func TestPart6WriteReport(t *testing.T) {
	t.Run("header, sorted rows, decimal size and UTC RFC 3339 nanoseconds", func(t *testing.T) {
		var buf bytes.Buffer
		if err := backup.WriteReport(&buf, reportManifest(), nil); err != nil {
			t.Fatalf("WriteReport: %v", err)
		}
		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		want := []string{
			"path,size,mtime,sha256,note",
			`"a,quoted ""x""",0,2026-03-04T05:06:07.123456001Z,` + sum("").String() + ",",
			"a/first.bin,7,2026-03-04T04:06:07.123456Z," + sum("f").String() + ",",
			"b/z.txt,1234567,2026-03-04T05:06:07.123456Z," + sum("z").String() + ",",
		}
		if diff := cmp.Diff(want, lines); diff != "" {
			t.Errorf("report lines (-want +got):\n%s", diff)
		}
	})

	t.Run("notes fill the note column; absent notes are empty", func(t *testing.T) {
		var buf bytes.Buffer
		notes := map[string]string{"b/z.txt": "needs review", "a/first.bin": "renamed, see #4"}
		if err := backup.WriteReport(&buf, reportManifest(), notes); err != nil {
			t.Fatalf("WriteReport: %v", err)
		}
		rows, err := backup.ReadReport(&buf)
		if err != nil {
			t.Fatalf("ReadReport: %v", err)
		}
		got := make(map[string]string, len(rows))
		for _, r := range rows {
			got[r.Path] = r.Note
		}
		want := map[string]string{"b/z.txt": "needs review", "a/first.bin": "renamed, see #4", `a,quoted "x"`: ""}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("notes (-want +got):\n%s", diff)
		}
	})

	t.Run("note for a path not in the manifest fails and writes nothing", func(t *testing.T) {
		var buf bytes.Buffer
		err := backup.WriteReport(&buf, reportManifest(), map[string]string{"ghost": "x"})
		if err == nil {
			t.Fatal("WriteReport succeeded with a note for an unknown path")
		}
		if buf.Len() != 0 {
			t.Errorf("wrote %d bytes before failing; want none", buf.Len())
		}
		buf.Reset()
		if err := backup.WriteReport(&buf, reportManifest(), map[string]string{"a/first.bin": "ok"}); err != nil {
			t.Fatalf("WriteReport rejected a valid note: %v", err)
		}
	})

	t.Run("empty manifest writes only the header", func(t *testing.T) {
		var buf bytes.Buffer
		if err := backup.WriteReport(&buf, nil, nil); err != nil {
			t.Fatalf("WriteReport: %v", err)
		}
		if got, want := buf.String(), "path,size,mtime,sha256,note\n"; got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})
}

func TestPart6ReadReport(t *testing.T) {
	var buf bytes.Buffer
	if err := backup.WriteReport(&buf, reportManifest(), map[string]string{"b/z.txt": "ok"}); err != nil {
		t.Fatalf("setup WriteReport: %v", err)
	}
	written := buf.String()

	t.Run("round trip recovers every field", func(t *testing.T) {
		rows, err := backup.ReadReport(strings.NewReader(written))
		if err != nil {
			t.Fatalf("ReadReport: %v", err)
		}
		want := []backup.ReportRow{
			{Path: `a,quoted "x"`, Size: 0, ModTime: stampU.Add(time.Nanosecond), Digest: sum("")},
			{Path: "a/first.bin", Size: 7, ModTime: stampU.Add(-time.Hour), Digest: sum("f")},
			{Path: "b/z.txt", Size: 1234567, ModTime: stampU, Digest: sum("z"), Note: "ok"},
		}
		if diff := cmp.Diff(want, rows, cmp.Comparer(func(a, b time.Time) bool { return a.Equal(b) })); diff != "" {
			t.Errorf("rows (-want +got):\n%s", diff)
		}
	})

	t.Run("BOM and CRLF are accepted", func(t *testing.T) {
		crlf := "\uFEFF" + strings.ReplaceAll(written, "\n", "\r\n")
		rows, err := backup.ReadReport(strings.NewReader(crlf))
		if err != nil {
			t.Fatalf("ReadReport(BOM+CRLF): %v", err)
		}
		if len(rows) != 3 || rows[2].Note != "ok" {
			t.Errorf("rows = %+v, want 3 rows with note ok on the last", rows)
		}
	})

	bad := []struct {
		name    string
		in      string
		wantErr string
	}{
		{name: "empty input", in: "", wantErr: ""},
		{name: "header mismatch", in: "path,size,mtime,digest,note\n", wantErr: "header"},
		{name: "short row names the line", in: "path,size,mtime,sha256,note\nx,1,2026-03-04T05:06:07Z\n", wantErr: "line 2"},
		{name: "bad size", in: "path,size,mtime,sha256,note\nx,big,2026-03-04T05:06:07Z," + sum("x").String() + ",\n", wantErr: "size"},
		{name: "bad mtime", in: "path,size,mtime,sha256,note\nx,1,yesterday," + sum("x").String() + ",\n", wantErr: "mtime"},
		{name: "bad digest", in: "path,size,mtime,sha256,note\nx,1,2026-03-04T05:06:07Z,nothex,\n", wantErr: "sha256"},
	}
	for _, tc := range bad {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			_, err := backup.ReadReport(strings.NewReader(tc.in))
			if err == nil {
				t.Fatal("ReadReport succeeded, want error")
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v, want one mentioning %q", err, tc.wantErr)
			}
		})
	}

	t.Run("reader errors are returned", func(t *testing.T) {
		boom := errors.New("disk on fire")
		_, err := backup.ReadReport(errReader{err: boom})
		if !errors.Is(err, boom) {
			t.Fatalf("ReadReport error = %v, want to wrap %v", err, boom)
		}
	})
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }
