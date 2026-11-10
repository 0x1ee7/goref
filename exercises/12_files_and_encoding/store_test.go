package backup_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	backup "goref/exercises/12_files_and_encoding"
)

func TestPart4WriteFileAtomic(t *testing.T) {
	t.Run("creates, then replaces, leaving no temporary files", func(t *testing.T) {
		dir := t.TempDir()
		name := filepath.Join(dir, "state.json")
		if err := backup.WriteFileAtomic(name, []byte("v1"), 0o644); err != nil {
			t.Fatalf("first write: %v", err)
		}
		if err := backup.WriteFileAtomic(name, []byte("v2"), 0o644); err != nil {
			t.Fatalf("replace: %v", err)
		}
		b, err := os.ReadFile(name)
		if err != nil || string(b) != "v2" {
			t.Fatalf("content = %q, %v; want \"v2\"", b, err)
		}
		if diff := cmp.Diff([]string{"state.json"}, dirNames(t, dir)); diff != "" {
			t.Errorf("directory after writes (-want +got):\n%s", diff)
		}
	})

	t.Run("missing parent directory wraps fs.ErrNotExist", func(t *testing.T) {
		err := backup.WriteFileAtomic(filepath.Join(t.TempDir(), "absent", "x.json"), []byte("x"), 0o644)
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("WriteFileAtomic error = %v, want fs.ErrNotExist", err)
		}
	})

	t.Run("failed rename removes the temporary file", func(t *testing.T) {
		dir := t.TempDir()
		if err := backup.WriteFileAtomic(filepath.Join(dir, "ok.json"), []byte("ok"), 0o644); err != nil {
			t.Fatalf("setup write: %v", err)
		}
		target := filepath.Join(dir, "target")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := backup.WriteFileAtomic(target, []byte("x"), 0o644); err == nil {
			t.Fatal("WriteFileAtomic onto a directory succeeded")
		}
		if diff := cmp.Diff([]string{"ok.json", "target"}, dirNames(t, dir)); diff != "" {
			t.Errorf("leftover files after failed rename (-want +got):\n%s", diff)
		}
	})
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

const goodDoc = `{"format":1,"created":"2026-03-04T05:06:07.123456Z","entries":{"a.txt":{"size":5,"mtime":"2026-03-04T05:06:07.123456Z","sha256":"` +
	"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" + `"}}}`

// saveGood writes a valid document so each negative case starts from a
// known-good state; a failure here is reported as a setup failure.
func saveGood(t *testing.T, name string) {
	t.Helper()
	doc := backup.Document{
		Created: stampU,
		Entries: backup.Manifest{"a.txt": {Size: 5, ModTime: stampU, Digest: sum("hello")}},
	}
	if err := backup.SaveManifest(name, doc); err != nil {
		t.Fatalf("setup SaveManifest: %v", err)
	}
}

func TestPart4DocumentRoundTrip(t *testing.T) {
	t.Run("round trip keeps created, entries and digests", func(t *testing.T) {
		name := filepath.Join(t.TempDir(), "manifest.json")
		want := backup.Document{
			Created: stampU,
			Entries: backup.Manifest{
				"a.txt":     {Size: 5, ModTime: stampU, Digest: sum("hello")},
				"sub/b.bin": {Size: 0, ModTime: stampU, Digest: sum("")},
			},
		}
		if err := backup.SaveManifest(name, want); err != nil {
			t.Fatalf("SaveManifest: %v", err)
		}
		got, err := backup.LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest: %v", err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("document mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("zero created is omitted from the file", func(t *testing.T) {
		name := filepath.Join(t.TempDir(), "manifest.json")
		doc := backup.Document{Entries: backup.Manifest{"a.txt": {Size: 5, ModTime: stampU, Digest: sum("hello")}}}
		if err := backup.SaveManifest(name, doc); err != nil {
			t.Fatalf("SaveManifest: %v", err)
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), `"created"`) {
			t.Errorf("zero Created was written:\n%s", raw)
		}
		if !strings.Contains(string(raw), `"format": 1`) {
			t.Errorf("format field missing:\n%s", raw)
		}
		got, err := backup.LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest: %v", err)
		}
		if !got.Created.IsZero() {
			t.Errorf("Created = %v, want zero", got.Created)
		}
	})

	t.Run("nil entries round trip as empty", func(t *testing.T) {
		name := filepath.Join(t.TempDir(), "manifest.json")
		if err := backup.SaveManifest(name, backup.Document{Created: stampU}); err != nil {
			t.Fatalf("SaveManifest: %v", err)
		}
		got, err := backup.LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest: %v", err)
		}
		if diff := cmp.Diff(backup.Document{Created: stampU}, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("document mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("missing file wraps fs.ErrNotExist", func(t *testing.T) {
		_, err := backup.LoadManifest(filepath.Join(t.TempDir(), "nope.json"))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("LoadManifest error = %v, want fs.ErrNotExist", err)
		}
	})
}

func TestPart4LoadStrictness(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr string // substring that must appear in the error
	}{
		{name: "unknown top-level field", content: strings.Replace(goodDoc, `"entries"`, `"extra":true,"entries"`, 1), wantErr: "extra"},
		{name: "unsupported format", content: strings.Replace(goodDoc, `"format":1`, `"format":2`, 1), wantErr: "format"},
		{name: "trailing data after the document", content: goodDoc + ` {}`, wantErr: "trailing"},
		{name: "malformed digest", content: strings.Replace(goodDoc, "2cf24dba", "zzzz4dba", 1), wantErr: "sha256"},
		{name: "short digest", content: strings.Replace(goodDoc, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", "2cf2", 1), wantErr: "sha256"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := filepath.Join(t.TempDir(), "manifest.json")
			saveGood(t, name)
			if err := os.WriteFile(name, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := backup.LoadManifest(name)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("LoadManifest error = %v, want one mentioning %q", err, tc.wantErr)
			}
		})
	}
	t.Run("the good document loads", func(t *testing.T) {
		name := filepath.Join(t.TempDir(), "manifest.json")
		if err := os.WriteFile(name, []byte(goodDoc), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := backup.LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest(good): %v", err)
		}
		if got.Entries["a.txt"].Digest != sum("hello") {
			t.Errorf("digest = %s, want %s", got.Entries["a.txt"].Digest, sum("hello"))
		}
	})
}

func TestPart4DigestJSON(t *testing.T) {
	hexOf := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	t.Run("marshals as lowercase hex string", func(t *testing.T) {
		b, err := json.Marshal(sum("hello"))
		if err != nil {
			t.Fatal(err)
		}
		if want := `"` + hexOf + `"`; string(b) != want {
			t.Errorf("json.Marshal = %s, want %s", b, want)
		}
		if len(hexOf) != 64 || hexOf != strings.ToLower(hexOf) {
			t.Errorf("String() = %q, want 64 lowercase hex characters", hexOf)
		}
	})
	t.Run("parses upper and lower case", func(t *testing.T) {
		for _, s := range []string{hexOf, strings.ToUpper(hexOf)} {
			d, err := backup.ParseDigest(s)
			if err != nil || d != sum("hello") {
				t.Errorf("ParseDigest(%q) = %s, %v", s, d, err)
			}
		}
	})
	bad := []string{`"` + hexOf[:63] + `"`, `"` + hexOf + `00"`, `"g` + hexOf[1:] + `"`, `12`}
	for _, in := range bad {
		t.Run("rejects "+in, func(t *testing.T) {
			var ok backup.Digest
			if err := json.Unmarshal([]byte(`"`+hexOf+`"`), &ok); err != nil {
				t.Fatalf("canonical hex rejected: %v", err)
			}
			var d backup.Digest
			err := json.Unmarshal([]byte(in), &d)
			if err == nil {
				t.Errorf("Unmarshal(%s) succeeded with %s, want error", in, d)
			}
		})
	}
}

func TestPart5StreamRecords(t *testing.T) {
	const rec = `{"path":"%s","size":%d,"mtime":"2026-03-04T05:06:07.123456Z","sha256":"%s"}`
	collect := func(r io.Reader) ([]backup.Record, error) {
		var out []backup.Record
		for rec, err := range backup.StreamRecords(r) {
			if err != nil {
				return out, err
			}
			out = append(out, rec)
		}
		return out, nil
	}

	t.Run("streams a large array in order", func(t *testing.T) {
		const n = 20_000
		var b strings.Builder
		b.WriteString("[")
		for i := range n {
			if i > 0 {
				b.WriteString(",\n")
			}
			fmt.Fprintf(&b, rec, fmt.Sprintf("dir/f%05d.txt", i), i, sum("x").String())
		}
		b.WriteString("]\n")
		got, err := collect(strings.NewReader(b.String()))
		if err != nil {
			t.Fatalf("StreamRecords: %v", err)
		}
		if len(got) != n {
			t.Fatalf("got %d records, want %d", len(got), n)
		}
		if got[0].Path != "dir/f00000.txt" || got[n-1].Path != fmt.Sprintf("dir/f%05d.txt", n-1) || got[n-1].Size != n-1 {
			t.Errorf("order or fields wrong: first %+v last %+v", got[0], got[n-1])
		}
	})

	t.Run("record fields are flat in the JSON object", func(t *testing.T) {
		in := fmt.Sprintf("["+rec+"]", "a b.txt", 5, sum("hello").String())
		got, err := collect(strings.NewReader(in))
		if err != nil {
			t.Fatalf("StreamRecords: %v", err)
		}
		want := []backup.Record{{Path: "a b.txt", Entry: backup.Entry{Size: 5, ModTime: stampU, Digest: sum("hello")}}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("records mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("early break stops decoding", func(t *testing.T) {
		const n = 20_000
		var b strings.Builder
		b.WriteString("[")
		for i := range n {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, rec, fmt.Sprintf("f%d", i), i, sum("x").String())
		}
		b.WriteString("]")
		total := b.Len()
		cr := &countingReader{r: strings.NewReader(b.String())}
		seen := 0
		for _, err := range backup.StreamRecords(cr) {
			if err != nil {
				t.Fatalf("StreamRecords: %v", err)
			}
			seen++
			if seen == 3 {
				break
			}
		}
		if seen != 3 {
			t.Fatalf("saw %d records before break, want 3", seen)
		}
		if cr.n > total/4 {
			t.Errorf("read %d of %d bytes for 3 records: input is not streamed", cr.n, total)
		}
	})

	errCases := []struct {
		name     string
		in       string
		wantRecs int
	}{
		{name: "top-level object instead of array", in: `{"path":"a"}`, wantRecs: 0},
		{name: "empty input", in: ``, wantRecs: 0},
		{name: "unknown field in element", in: "[" + fmt.Sprintf(rec, "ok", 1, sum("x").String()) + `,{"path":"b","bogus":1}]`, wantRecs: 1},
		{name: "truncated element", in: "[" + fmt.Sprintf(rec, "ok", 1, sum("x").String()) + `,{"path":`, wantRecs: 1},
		{name: "data after closing bracket", in: "[" + fmt.Sprintf(rec, "ok", 1, sum("x").String()) + `] []`, wantRecs: 1},
	}
	for _, tc := range errCases {
		t.Run("error: "+tc.name, func(t *testing.T) {
			got, err := collect(strings.NewReader(tc.in))
			if err == nil {
				t.Fatalf("StreamRecords succeeded with %d records, want an error", len(got))
			}
			if len(got) != tc.wantRecs {
				t.Errorf("records before error = %d, want %d", len(got), tc.wantRecs)
			}
		})
	}
}

type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}
