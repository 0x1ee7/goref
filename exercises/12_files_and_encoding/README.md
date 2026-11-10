# 12 — Files & Encoding

## Scenario

You are building the core of a small backup/sync tool. It scans a source tree
honouring ignore patterns, records a manifest of `{relative slash path: size,
mtime, sha256}`, diffs two manifests, applies the diff to a mirror without
letting writes escape it, persists manifests atomically as strictly-decoded
JSON, exports a CSV report with awkward values, streams records out of a large
JSON array, compresses archives in a streaming fashion, and writes a versioned
binary snapshot header.

Everything lives in package `backup`. Tests use `t.TempDir()`, `fstest.MapFS`
and loopback buffers; they never touch the real filesystem outside temp dirs.

## Conventions

- Manifest keys are **relative slash-separated paths** (`"pkg/mod.go"`), even on
  Windows. Use `path`, not `filepath`, for keys.
- Times are UTC and truncated to microseconds (`time.Time` values compare with
  `Equal`, not `==`).
- Errors wrap sentinels or the standard errors (`fs.ErrNotExist`,
  `path.ErrBadPattern`, …) with `%w` so callers can use `errors.Is` / `errors.As`.
  Do not invent new sentinels unless the spec says so.
- The package must not panic on any input and must not leak goroutines or files.

## Part 1 — Tree scan, ignore rules, and manifest paths

```go
type Entry struct {
    Size    int64     `json:"size"`
    ModTime time.Time `json:"mtime"`
    Digest  Digest    `json:"sha256"`
}
type Manifest map[string]Entry

func (m Manifest) Paths() iter.Seq[string] // keys in sorted order

func Scan(fsys fs.FS, ignore ...string) (Manifest, error)
func ParseIgnore(r io.Reader) ([]string, error)
```

`Scan` walks `fsys` from `"."` (`fs.WalkDir`). Only regular files are included;
symlinks and other non-regular entries are **skipped, never followed**. Each
entry's `Size` is the number of bytes hashed, `ModTime` is the file's mtime in
UTC truncated to microseconds, and `Digest` is the SHA-256 of the content,
streamed (never read whole into memory). The digest itself is Part 2's `Hash`.

Ignore rules, applied to each file's relative path:

- A pattern **without** `/` matches if **any path component** (file or directory
  name, at any depth) matches it via `path.Match`. `"*.tmp"` drops `a.tmp` and
  `x/y/b.tmp`. `"__pycache__"` drops everything under any `__pycache__`
  directory, so a matching directory prunes its whole subtree.
- A pattern **with** `/` is anchored at the root and must match the **whole**
  relative path. `**` as a whole segment matches zero or more segments; a single
  `*` never crosses `/`. `"logs/**"` drops `logs/a.log` and `logs/deep/b.log` but
  keeps `other/logs/c.log`; `"logs/*"` drops `logs/a.log` but keeps
  `logs/deep/b.log`.
- A malformed pattern (including the empty string) is an error wrapping
  `path.ErrBadPattern`.

`ParseIgnore` reads one pattern per line: surrounding whitespace is trimmed,
blank lines and lines beginning with `#` are skipped. A line longer than
`bufio`'s default token limit (64 KiB) is an error wrapping `bufio.ErrTooLong`.

`Manifest.Paths` yields keys sorted, and is usable with `range` over a function.

## Part 2 — Directory scan, symlinks, and hashing

```go
var ErrNotDirectory = errors.New("not a directory")

func ScanDir(root string, ignore ...string) (Manifest, error)
func Hash(r io.Reader) (Digest, int64, error)
```

`ScanDir` is `Scan` over an operating-system directory, opened with
`os.OpenRoot` so nothing outside `root` can be reached, even through a symlink
that points out of it.

- Missing `root` → an error satisfying `errors.Is(err, fs.ErrNotExist)`.
- `root` exists but is a regular file → an error wrapping `ErrNotDirectory`.
- A symlink inside `root` pointing at a file or a directory outside it is
  invisible to the scan.

`Hash` streams `r` through SHA-256 and returns the digest and the number of
bytes read. On a read error it returns the error with the zero `Digest`.

## Part 3 — Diff, path safety, and apply

```go
type ManifestDiff struct {
    Added    []string
    Removed  []string
    Modified []string
}
func (d ManifestDiff) Empty() bool

func Diff(old, new Manifest) ManifestDiff
func CheckRelPath(name string) error
type ApplyResult struct {
    Copied  []string // added and modified paths copied from the source
    Deleted []string // removed paths actually deleted from the destination
}
func Apply(d ManifestDiff, src fs.FS, dst *os.Root) (ApplyResult, error)

var ErrUnsafePath = errors.New("unsafe path")
```

`Diff`: `Added` are keys only in `new`, `Removed` keys only in `old`, `Modified`
keys in both whose `Size` or `Digest` differ. An **mtime-only** change is not a
modification. All three slices are sorted; `Empty` is true when all are empty.

`CheckRelPath` accepts a relative, slash-separated, `fs.ValidPath`-clean name.
It rejects, wrapping `ErrUnsafePath`: the empty string, `"."`, absolute paths,
`..` and `.` segments, empty segments, backslashes, and colons. `"..foo"` is a
legal file name.

`Apply` must:

- Validate **every** path in all three slices with `CheckRelPath` before
  touching the disk. A failure wraps `ErrUnsafePath`.
- Copy `Added` and `Modified` files from `src` into `dst`, creating missing
  parent directories, and set the copied file's mtime to the source mtime.
- Delete `Removed` files. A file already absent is not an error and is **not**
  reported in `Deleted`.
- After a deletion, remove parent directories that became empty, climbing up to
  but **not including** the root. Non-empty directories are left alone.
- Propagate a missing source file as an error wrapping `fs.ErrNotExist`.
- Refuse to write through a symlink that escapes `dst`. Every access goes through
  the `*os.Root`.

## Part 4 — Atomic writes, strict JSON, and digest encoding

```go
func WriteFileAtomic(name string, data []byte, perm fs.FileMode) error

type Document struct {
    Created time.Time `json:"created,omitzero"`
    Entries Manifest  `json:"entries"`
}
func SaveManifest(name string, doc Document) error
func LoadManifest(name string) (Document, error)

type Digest [sha256.Size]byte
func ParseDigest(s string) (Digest, error)
func (d Digest) String() string
func (d Digest) MarshalJSON() ([]byte, error)
func (d *Digest) UnmarshalJSON(b []byte) error
```

`WriteFileAtomic` writes a temporary file **in the same directory** as `name`,
syncs it, sets `perm`, and renames it over `name`. On any failure the temporary
file is removed and the error returned; an existing `name` is left untouched.
A missing parent directory yields an error satisfying `fs.ErrNotExist`.

`SaveManifest` writes indented JSON with a leading `"format": 1` field and a
trailing newline, via `WriteFileAtomic` with mode `0644`.

`LoadManifest` decodes **strictly**: unknown fields are an error; `format` must
be `1`; malformed digests are errors; trailing data after the document is an
error. A missing file wraps `fs.ErrNotExist`.

`Digest` is a value type and JSON-encodes as a 64-character lowercase hex
string. `ParseDigest` accepts either case and rejects any other length or
non-hex character. `String` is lowercase. `UnmarshalJSON` rejects non-string
JSON and invalid hex.

## Part 5 — Streaming a JSON array with iterators

```go
type Record struct {
    Path string `json:"path"`
    Entry
}
func StreamRecords(r io.Reader) iter.Seq2[Record, error]
```

`StreamRecords` decodes a JSON array of records from `r`, one element at a time,
without loading the array into memory. Each record is yielded as
`(rec, nil)`. The `Entry` is embedded, so each element is flat:
`{"path":…,"size":…,"mtime":…,"sha256":…}`. Unknown fields are an error.

Errors are yielded as `(Record{}, err)` and the iteration **stops**. Errors
include: a top-level value that is not an array, malformed elements, an
unterminated array, empty input, and data after the closing bracket.

If the consumer `break`s early, decoding stops and **no** error is yielded.

## Part 6 — CSV report with awkward values

```go
type ReportRow struct {
    Path    string
    Size    int64
    ModTime time.Time
    Digest  Digest
    Note    string
}
func WriteReport(w io.Writer, m Manifest, notes map[string]string) error
func ReadReport(r io.Reader) ([]ReportRow, error)
```

`WriteReport` writes the header `path,size,mtime,sha256,note`, then one row per
manifest entry sorted by path (`encoding/csv` quoting applies, so paths with
commas or quotes round-trip). `size` is decimal; `mtime` is RFC 3339 with
nanoseconds in UTC; `note` is `notes[path]`. A key in `notes` that is not in
`m` is an error, and nothing may be written in that case.

`ReadReport` must accept:

- a leading UTF-8 byte order mark;
- CRLF line endings.

The header must match exactly. Every row must have five fields. Negative sizes,
invalid mtimes, and invalid digests are errors. Each error names the line
number, via `csv.Reader.FieldPos`.

## Part 7 — Gzip streams and a versioned binary snapshot

```go
var ErrCorrupt = errors.New("corrupt gzip stream")

func Compress(dst io.Writer, src io.Reader, level int) (int64, error)
func Decompress(dst io.Writer, src io.Reader) (int64, error)
func VerifyGzip(src io.Reader) error

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

type Snapshot struct {
    Version    uint16    // set by ReadSnapshot; ignored by WriteSnapshot
    Flags      uint16
    EntryCount uint32
    CreatedAt  time.Time // UTC, microseconds; zero for version 1
    Payload    []byte
}
func WriteSnapshot(w io.Writer, s Snapshot) error
func ReadSnapshot(r io.Reader) (Snapshot, error)
```

`Compress` streams `src` into a gzip writer at `level` (any value
`gzip.NewWriterLevel` accepts; an invalid level is an error that does **not**
wrap `ErrCorrupt`) and returns the uncompressed byte count. The output is
reproducible: no name in the header, zero mtime, so identical input gives
identical bytes.

`Decompress` streams a gzip stream (including concatenated members) into `dst`
and returns the uncompressed byte count. Malformed input (bad header, bad
checksum, truncation, invalid deflate data, empty input) wraps `ErrCorrupt`.
Errors from `dst` or from the source reader are **not** wrapped with
`ErrCorrupt`. The distinction matters: a full disk is not a corrupt archive.

`VerifyGzip` decompresses to `io.Discard` and returns nil for a complete valid
stream, or an error wrapping `ErrCorrupt`.

`WriteSnapshot` writes the version 2 layout. Integers are big-endian:

| Version | Layout |
|---|---|
| 1 | magic `"BKSN"` (4) · version u16 (`1`) · flags u16 · entry_count u32 — 12 bytes |
| 2 | v1 fields with version `2`, then `created_us` i64 (Unix microseconds) — 20 bytes |

Then the payload, then a **CRC-32C (Castagnoli) trailer** over every byte before
it. A zero `CreatedAt` is an error.

`ReadSnapshot` reads all of `r` and checks, in order:

1. fewer than 4 bytes, or fewer than 6 bytes → `ErrSnapshotTruncated`;
2. magic other than `"BKSN"` → `ErrSnapshotMagic`;
3. version other than 1 or 2 → `ErrSnapshotVersion`;
4. fewer bytes than the header plus trailer → `ErrSnapshotTruncated`;
5. checksum mismatch → `ErrSnapshotChecksum`.

For version 1, `CreatedAt` is the zero time. The payload is everything between
the header and the trailer, byte for byte.

## Concepts exercised

- `io/fs`: `fs.FS`, `fs.WalkDir`, `fs.SkipDir`, `fs.DirEntry.Type`, `fstest.MapFS`
- `os.Root` (Go 1.24+): traversal-safe `Open`, `Mkdir`, `Remove`, `Chtimes`, and `Root.FS`
- `path` vs `filepath`; `path.Match` and segment-wise glob matching
- Streaming hashes with `crypto/sha256`; `io.Copy` vs `io.ReadAll`
- Iterators: `iter.Seq`, `iter.Seq2`, `slices.Values`, `maps.Keys`, early `break`
- `encoding/json`: `Marshaler`/`Unmarshaler` on value types, `DisallowUnknownFields`, `json.Decoder.Token` and `More`, `omitzero`, embedded structs
- Atomic writes: `os.CreateTemp` in the target directory, `Sync`, `Rename`
- `encoding/csv`: `FieldsPerRecord`, `FieldPos`, quoting, BOM and CRLF handling
- `compress/gzip`: streaming, reproducible headers, distinguishing source, destination and corrupt-input errors with `%w: %w`
- `encoding/binary` with explicit byte order; `hash/crc32` (Castagnoli)
- Error design: sentinels vs wrapped detail, `errors.Is` across layers, not wrapping the wrong cause
- `testing`: `t.TempDir`, `t.Skipf` for environments without symlink privilege, table-driven tests, `go-cmp` with `cmpopts.EquateEmpty` and custom comparers

## Run

```
go test ./exercises/12_files_and_encoding/
go test ./exercises/12_files_and_encoding/ -run Part2 -v
go run ./tools/solutions ./exercises/12_files_and_encoding/
```

Symlink tests (`TestPart2ScanDirSymlinksStayInside`, and the symlink subtest in
`TestPart3Apply`) skip themselves on systems where the process cannot create
symlinks (Windows without Developer Mode or elevation). Enable Developer Mode to
exercise them.

Run `-race` on Linux or macOS (it needs cgo and is unavailable on the Windows
dev box). The exercise has no intended concurrency, so `-race` should be clean.

## Stretch (untested)

- Make `Scan` run hashing on a bounded worker pool with `errgroup.SetLimit`. Keep the returned manifest deterministic and measure the speed-up.
- Replace the bare-name pattern check with a precompiled ignore matcher (`ignoreSet`) and benchmark it against per-file `path.Match` calls with `b.Loop()`.
- Make `StreamRecords` also emit the byte offset of each record via `json.Decoder.InputOffset`.
- Add `CreatedAt` as a required `flags` bit and reject version 1 snapshots at a new call site. Decide where the policy belongs.
- Sanitise CSV cells that start with `=`, `+`, `-`, `@` to block spreadsheet formula injection. Decide where that belongs, and whether `ReadReport` should refuse them.
