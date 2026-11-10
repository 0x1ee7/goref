package backup_test

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"testing/iotest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	backup "goref/exercises/12_files_and_encoding"
)

// stamp carries sub-microsecond precision: manifests keep only microseconds,
// so the expected values in tests use stampU.
var (
	stamp  = time.Date(2026, 3, 4, 5, 6, 7, 123_456_789, time.UTC)
	stampU = time.Date(2026, 3, 4, 5, 6, 7, 123_456_000, time.UTC)
)

func sum(s string) backup.Digest { return backup.Digest(sha256.Sum256([]byte(s))) }

func entryOf(content string, mtime time.Time) backup.Entry {
	return backup.Entry{Size: int64(len(content)), ModTime: mtime, Digest: sum(content)}
}

func mapFile(content string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(content), ModTime: stamp}
}

// writeTree materialises files (slash paths) under root with mtime = stamp.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
}

func openRoot(t *testing.T, dir string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

// listFiles returns the sorted slash paths of regular files in fsys.
func listFiles(t *testing.T, fsys fs.FS) []string {
	t.Helper()
	var out []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

func TestPart1Scan(t *testing.T) {
	cases := []struct {
		name   string
		fsys   fstest.MapFS
		ignore []string
		want   backup.Manifest
	}{
		{
			name: "walks nested directories with sizes and digests",
			fsys: fstest.MapFS{
				"a.txt":              mapFile("alpha"),
				"dir/b.txt":          mapFile("beta"),
				"dir/deep/empty.bin": mapFile(""),
			},
			want: backup.Manifest{
				"a.txt":              entryOf("alpha", stampU),
				"dir/b.txt":          entryOf("beta", stampU),
				"dir/deep/empty.bin": entryOf("", stampU),
			},
		},
		{
			name: "symlinks are skipped, not followed",
			fsys: fstest.MapFS{
				"real.txt": mapFile("x"),
				"link.txt": {Data: []byte("real.txt"), Mode: fs.ModeSymlink, ModTime: stamp},
			},
			want: backup.Manifest{"real.txt": entryOf("x", stampU)},
		},
		{
			name: "bare pattern matches any path component at any depth",
			fsys: fstest.MapFS{
				"a.tmp":     mapFile("1"),
				"x/y/b.tmp": mapFile("2"),
				"keep.txt":  mapFile("3"),
				"x/keep.go": mapFile("4"),
			},
			ignore: []string{"*.tmp"},
			want: backup.Manifest{
				"keep.txt":  entryOf("3", stampU),
				"x/keep.go": entryOf("4", stampU),
			},
		},
		{
			name: "bare directory name prunes its whole subtree",
			fsys: fstest.MapFS{
				"pkg/__pycache__/m.pyc":    mapFile("pyc"),
				"pkg/__pycache__/deep/z.o": mapFile("zo"),
				"pkg/mod.go":               mapFile("go"),
			},
			ignore: []string{"__pycache__"},
			want:   backup.Manifest{"pkg/mod.go": entryOf("go", stampU)},
		},
		{
			name: "slash pattern is anchored at the root and ** spans directories",
			fsys: fstest.MapFS{
				"logs/a.log":       mapFile("a"),
				"logs/deep/b.log":  mapFile("b"),
				"other/logs/c.log": mapFile("c"),
			},
			ignore: []string{"logs/**"},
			want:   backup.Manifest{"other/logs/c.log": entryOf("c", stampU)},
		},
		{
			name: "single star does not cross a slash",
			fsys: fstest.MapFS{
				"logs/a.log":      mapFile("a"),
				"logs/deep/b.log": mapFile("b"),
			},
			ignore: []string{"logs/*"},
			want:   backup.Manifest{"logs/deep/b.log": entryOf("b", stampU)},
		},
		{
			name: "several patterns combine",
			fsys: fstest.MapFS{
				"x.tmp":          mapFile("t"),
				"build/out/app":  mapFile("b"),
				"docs/a.bak":     mapFile("a"),
				"docs/sub/a.bak": mapFile("s"),
				"docs/readme.md": mapFile("r"),
			},
			ignore: []string{"*.tmp", "build/**", "docs/*.bak"},
			want: backup.Manifest{
				"docs/sub/a.bak": entryOf("s", stampU),
				"docs/readme.md": entryOf("r", stampU),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := backup.Scan(tc.fsys, tc.ignore...)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Scan mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart1BadPattern(t *testing.T) {
	fsys := fstest.MapFS{"a": mapFile("x")}
	for _, pat := range []string{"[", "logs/[", ""} {
		t.Run(pat, func(t *testing.T) {
			_, err := backup.Scan(fsys, pat)
			if !errors.Is(err, path.ErrBadPattern) {
				t.Fatalf("Scan(ignore=%q) error = %v, want one wrapping path.ErrBadPattern", pat, err)
			}
		})
	}
}

func TestPart1Paths(t *testing.T) {
	m := backup.Manifest{"c/d": entryOf("3", stampU), "a": entryOf("1", stampU), "b": entryOf("2", stampU)}
	t.Run("yields keys sorted", func(t *testing.T) {
		got := slices.Collect(m.Paths())
		if diff := cmp.Diff([]string{"a", "b", "c/d"}, got); diff != "" {
			t.Errorf("Paths mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("stops when the consumer breaks", func(t *testing.T) {
		n := 0
		for range m.Paths() {
			n++
			break
		}
		if n != 1 {
			t.Errorf("loop body ran %d times, want 1", n)
		}
	})
}

func TestPart1ParseIgnore(t *testing.T) {
	t.Run("trims, skips blank lines and comments", func(t *testing.T) {
		in := "# build junk\n\n  *.tmp  \r\nlogs/**\n   \n#x\n build/**\n"
		got, err := backup.ParseIgnore(strings.NewReader(in))
		if err != nil {
			t.Fatalf("ParseIgnore: %v", err)
		}
		if diff := cmp.Diff([]string{"*.tmp", "logs/**", "build/**"}, got); diff != "" {
			t.Errorf("ParseIgnore mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("line beyond the scanner token limit is an error", func(t *testing.T) {
		in := strings.Repeat("x", 70_000) + "\n"
		_, err := backup.ParseIgnore(strings.NewReader(in))
		if !errors.Is(err, bufio.ErrTooLong) {
			t.Fatalf("ParseIgnore error = %v, want one wrapping bufio.ErrTooLong", err)
		}
	})
}

func TestPart2ScanDir(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		ignore []string
		want   backup.Manifest
	}{
		{
			name:  "regular files under a real directory",
			files: map[string]string{"a.txt": "alpha", "sub/b.txt": "beta"},
			want: backup.Manifest{
				"a.txt":     entryOf("alpha", stampU),
				"sub/b.txt": entryOf("beta", stampU),
			},
		},
		{
			name:   "ignore patterns apply to the real tree",
			files:  map[string]string{"keep.txt": "k", "junk.tmp": "t", "logs/app.log": "l"},
			ignore: []string{"*.tmp", "logs/**"},
			want:   backup.Manifest{"keep.txt": entryOf("k", stampU)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, tc.files)
			got, err := backup.ScanDir(root, tc.ignore...)
			if err != nil {
				t.Fatalf("ScanDir: %v", err)
			}
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("ScanDir mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart2ScanDirSymlinksStayInside(t *testing.T) {
	outside := t.TempDir()
	writeTree(t, outside, map[string]string{"secret.txt": "s"})
	root := t.TempDir()
	writeTree(t, root, map[string]string{"inside.txt": "i"})
	want := backup.Manifest{"inside.txt": entryOf("i", stampU)}
	check := func(stage string) {
		t.Helper()
		got, err := backup.ScanDir(root)
		if err != nil {
			t.Fatalf("ScanDir (%s): %v", stage, err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ScanDir (%s) followed a symlink (-want +got):\n%s", stage, diff)
		}
	}
	check("plain tree")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "secret-link.txt")); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked-dir")); err != nil {
		t.Skipf("cannot create directory symlinks here: %v", err)
	}
	check("with symlinks")
}

func TestPart2ScanDirErrors(t *testing.T) {
	t.Run("missing root wraps fs.ErrNotExist", func(t *testing.T) {
		_, err := backup.ScanDir(filepath.Join(t.TempDir(), "absent"))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("ScanDir error = %v, want fs.ErrNotExist", err)
		}
	})
	t.Run("root that is a file is ErrNotDirectory", func(t *testing.T) {
		dir := t.TempDir()
		writeTree(t, dir, map[string]string{"f.txt": "x"})
		_, err := backup.ScanDir(filepath.Join(dir, "f.txt"))
		if !errors.Is(err, backup.ErrNotDirectory) {
			t.Fatalf("ScanDir error = %v, want backup.ErrNotDirectory", err)
		}
	})
}

// errAfter yields prefix and then fails with err.
type errAfter struct {
	prefix io.Reader
	err    error
}

func (r errAfter) Read(p []byte) (int, error) {
	n, err := r.prefix.Read(p)
	if err == io.EOF {
		return n, r.err
	}
	return n, err
}

func TestPart2Hash(t *testing.T) {
	big := make([]byte, 3<<20)
	for i := range big {
		big[i] = byte(i * 31)
	}
	t.Run("empty input", func(t *testing.T) {
		d, n, err := backup.Hash(strings.NewReader(""))
		if err != nil {
			t.Fatalf("Hash: %v", err)
		}
		if d != sum("") || n != 0 {
			t.Errorf("Hash(empty) = (%s, %d), want (%s, 0)", d, n, sum(""))
		}
	})
	t.Run("3 MiB streamed matches one-shot sha256", func(t *testing.T) {
		d, n, err := backup.Hash(strings.NewReader(string(big)))
		if err != nil {
			t.Fatalf("Hash: %v", err)
		}
		if want := backup.Digest(sha256.Sum256(big)); d != want || n != int64(len(big)) {
			t.Errorf("Hash = (%s, %d), want (%s, %d)", d, n, want, len(big))
		}
	})
	t.Run("tiny reads give the same digest", func(t *testing.T) {
		d, n, err := backup.Hash(iotest.OneByteReader(strings.NewReader("hello")))
		if err != nil {
			t.Fatalf("Hash: %v", err)
		}
		if d != sum("hello") || n != 5 {
			t.Errorf("Hash = (%s, %d), want (%s, 5)", d, n, sum("hello"))
		}
	})
	t.Run("read error is returned with the count so far", func(t *testing.T) {
		boom := errors.New("boom")
		d, n, err := backup.Hash(errAfter{prefix: strings.NewReader("abc"), err: boom})
		if !errors.Is(err, boom) {
			t.Fatalf("Hash error = %v, want boom", err)
		}
		if n != 3 || d != (backup.Digest{}) {
			t.Errorf("Hash on failure = (%s, %d), want (zero, 3)", d, n)
		}
	})
}

func TestPart3Diff(t *testing.T) {
	old := stampU.Add(-time.Hour)
	cases := []struct {
		name      string
		old, new  backup.Manifest
		want      backup.ManifestDiff
		wantEmpty bool
	}{
		{
			name:      "identical manifests are empty",
			old:       backup.Manifest{"a": entryOf("a", stampU)},
			new:       backup.Manifest{"a": entryOf("a", stampU)},
			wantEmpty: true,
		},
		{
			name: "added, removed and modified, all sorted",
			old: backup.Manifest{
				"z.txt": entryOf("z", stampU),
				"m.txt": entryOf("m1", stampU),
				"b.txt": entryOf("b", stampU),
			},
			new: backup.Manifest{
				"m.txt": entryOf("m2", stampU),
				"n.txt": entryOf("n", stampU),
				"a.txt": entryOf("a", stampU),
				"b.txt": entryOf("b", stampU),
			},
			want: backup.ManifestDiff{
				Added:    []string{"a.txt", "n.txt"},
				Removed:  []string{"z.txt"},
				Modified: []string{"m.txt"},
			},
		},
		{
			name: "mtime-only change is not a modification",
			old: backup.Manifest{
				"a": {Size: 1, ModTime: old, Digest: sum("a")},
			},
			new: backup.Manifest{
				"a": {Size: 1, ModTime: stampU, Digest: sum("a")},
				"b": entryOf("b", stampU),
			},
			want: backup.ManifestDiff{Added: []string{"b"}},
		},
		{
			name: "same size, different digest is a modification",
			old:  backup.Manifest{"a": entryOf("aaaa", stampU)},
			new:  backup.Manifest{"a": entryOf("bbbb", stampU)},
			want: backup.ManifestDiff{Modified: []string{"a"}},
		},
		{
			name: "empty old manifest adds everything",
			old:  nil,
			new:  backup.Manifest{"x": entryOf("x", stampU)},
			want: backup.ManifestDiff{Added: []string{"x"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := backup.Diff(tc.old, tc.new)
			if got.Empty() != tc.wantEmpty {
				t.Errorf("Empty() = %v, want %v", got.Empty(), tc.wantEmpty)
			}
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Diff mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart3EmptyPredicate(t *testing.T) {
	if !(backup.ManifestDiff{}).Empty() {
		t.Error("zero ManifestDiff: Empty() = false, want true")
	}
	if (backup.ManifestDiff{Removed: []string{"x"}}).Empty() {
		t.Error("diff with a removal: Empty() = true, want false")
	}
}

func TestPart3CheckRelPath(t *testing.T) {
	valid := []string{"a", "a/b.txt", "..foo", "a/..b/c", "dir with space/ünï 日本.txt", "a.b.c"}
	invalid := []string{"", "/abs", "../x", "a/../b", "a/./b", "a//b", ".", "..", "a/", "a\\b", "C:x", "a:b"}
	for _, name := range valid {
		t.Run("valid "+name, func(t *testing.T) {
			if err := backup.CheckRelPath(name); err != nil {
				t.Errorf("CheckRelPath(%q) = %v, want nil", name, err)
			}
		})
	}
	for _, name := range invalid {
		t.Run("invalid "+name, func(t *testing.T) {
			if err := backup.CheckRelPath(name); !errors.Is(err, backup.ErrUnsafePath) {
				t.Errorf("CheckRelPath(%q) = %v, want ErrUnsafePath", name, err)
			}
		})
	}
}

func TestPart3Apply(t *testing.T) {
	t.Run("copies, deletes, and prunes emptied directories", func(t *testing.T) {
		src := fstest.MapFS{
			"new.txt":     {Data: []byte("N"), ModTime: stamp},
			"dir/mod.txt": {Data: []byte("M2"), ModTime: stamp},
		}
		dst := t.TempDir()
		writeTree(t, dst, map[string]string{
			"dir/mod.txt":       "M1",
			"gone/a.txt":        "A",
			"gone/nested/b.txt": "B",
			"keep/c.txt":        "C",
		})
		d := backup.ManifestDiff{
			Added:    []string{"new.txt"},
			Modified: []string{"dir/mod.txt"},
			Removed:  []string{"gone/a.txt", "gone/nested/b.txt", "ghost.txt"},
		}
		got, err := backup.Apply(d, src, openRoot(t, dst))
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		want := backup.ApplyResult{
			Copied:  []string{"dir/mod.txt", "new.txt"},
			Deleted: []string{"gone/a.txt", "gone/nested/b.txt"},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ApplyResult mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]string{"dir/mod.txt", "keep/c.txt", "new.txt"}, listFiles(t, os.DirFS(dst))); diff != "" {
			t.Errorf("tree after apply (-want +got):\n%s", diff)
		}
		b, err := os.ReadFile(filepath.Join(dst, "dir", "mod.txt"))
		if err != nil || string(b) != "M2" {
			t.Errorf("dir/mod.txt = %q, %v; want \"M2\"", b, err)
		}
		info, err := os.Stat(filepath.Join(dst, "dir", "mod.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Truncate(time.Microsecond).Equal(stampU) {
			t.Errorf("copied mtime = %v, want source mtime %v", info.ModTime(), stampU)
		}
	})

	t.Run("traversal is rejected before anything changes", func(t *testing.T) {
		parent := t.TempDir()
		dst := filepath.Join(parent, "dst")
		if err := os.Mkdir(dst, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTree(t, dst, map[string]string{"victim.txt": "v"})
		src := fstest.MapFS{"ok.txt": {Data: []byte("x"), ModTime: stamp}}
		d := backup.ManifestDiff{Added: []string{"ok.txt", "../escape.txt"}, Removed: []string{"victim.txt"}}
		_, err := backup.Apply(d, src, openRoot(t, dst))
		if !errors.Is(err, backup.ErrUnsafePath) {
			t.Fatalf("Apply error = %v, want ErrUnsafePath", err)
		}
		if diff := cmp.Diff([]string{"victim.txt"}, listFiles(t, os.DirFS(dst))); diff != "" {
			t.Errorf("destination changed after rejected diff (-want +got):\n%s", diff)
		}
		if _, err := os.Stat(filepath.Join(parent, "escape.txt")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("escape.txt was created outside the root")
		}
	})

	t.Run("absolute and backslash names are rejected", func(t *testing.T) {
		dst := t.TempDir()
		writeTree(t, dst, map[string]string{"victim.txt": "v"})
		d := backup.ManifestDiff{Removed: []string{"victim.txt", `/etc/passwd`, `a\..\b`}}
		_, err := backup.Apply(d, fstest.MapFS{}, openRoot(t, dst))
		if !errors.Is(err, backup.ErrUnsafePath) {
			t.Fatalf("Apply error = %v, want ErrUnsafePath", err)
		}
		if diff := cmp.Diff([]string{"victim.txt"}, listFiles(t, os.DirFS(dst))); diff != "" {
			t.Errorf("victim deleted by rejected diff (-want +got):\n%s", diff)
		}
	})

	t.Run("removing an absent file is neither an error nor reported", func(t *testing.T) {
		dst := t.TempDir()
		got, err := backup.Apply(backup.ManifestDiff{Removed: []string{"nope/x.txt"}}, fstest.MapFS{}, openRoot(t, dst))
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if diff := cmp.Diff(backup.ApplyResult{}, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("ApplyResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("missing source file wraps fs.ErrNotExist", func(t *testing.T) {
		_, err := backup.Apply(backup.ManifestDiff{Added: []string{"x.txt"}}, fstest.MapFS{}, openRoot(t, t.TempDir()))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Apply error = %v, want fs.ErrNotExist", err)
		}
	})

	t.Run("symlink escape out of the destination is refused", func(t *testing.T) {
		outside := t.TempDir()
		dst := t.TempDir()
		root := openRoot(t, dst)
		src := fstest.MapFS{
			"fine.txt":       {Data: []byte("ok"), ModTime: stamp},
			"link/pwned.txt": {Data: []byte("x"), ModTime: stamp},
		}
		if _, err := backup.Apply(backup.ManifestDiff{Added: []string{"fine.txt"}}, src, root); err != nil {
			t.Fatalf("ordinary Apply: %v", err)
		}
		if err := os.Symlink(outside, filepath.Join(dst, "link")); err != nil {
			t.Skipf("cannot create directory symlinks here: %v", err)
		}
		if _, err := backup.Apply(backup.ManifestDiff{Added: []string{"link/pwned.txt"}}, src, root); err == nil {
			t.Fatal("Apply through an escaping symlink succeeded")
		}
		if _, err := os.Stat(filepath.Join(outside, "pwned.txt")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("file escaped through symlink: stat error = %v", err)
		}
	})
}
