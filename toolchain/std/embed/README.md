# embed: files compiled into the image (#928)

The compiler puts a directory tree or a file into the program's image; `embed` reads it back as a
read-only file system. Nothing is read from disk at run time. The directive and the diagnostics are in
toolchain/docs/TOOLING.md (section 3.5) and toolchain/docs/ERRORS.md (E004).

## API

- `Parse(blob str) !FS`: the file tree of a blob the compiler wrote; ErrFormat for anything else.
- `FS`: `Open(name) !dyn fs.File`, `ReadFile(name) !str`, `ReadDir(name) ![]fs.DirEntry`,
  `Stat(name) !fs.FileInfo`. These satisfy the fs package's FS, ReadFile, ReadDir, Stat and
  TreeFS shapes, so `fs.ReadFile`, `fs.WalkDir` and `fs.Glob` take an `FS` directly.
- `ErrFormat`: the fault of a blob that is not an embedded tree.

## Design notes

- The blob is written by the compiler (`embed_blob` in toolchain/compiler/parse.tin): one record per
  file, its name relative to the source directory, its size in decimal, and its bytes. Directory
  entries come in name order, so Parse can build each directory's listing in order and checks that
  it is sorted; a blob that is not in that order (or repeats a name) fails with ErrFormat.
- Error texts and modes follow Go's embed.FS, checked against a `go:embed` program: a missing name
  is "open NAME: file does not exist" (invalid names too), a file is read as a directory with
  "not a directory", a directory is read with a trailing slash, files are 0444 and directories 0555.
- `Read` ends with 0 and no fault, as the fs package's files do; `ReadDir(n)` hands out at most n
  entries and an empty slice at the end.
- Lookups are on a map of names and a map of directory listings; `ReadDir` and `Stat` are O(1) and
  the walk is linear in the tree. An FS is read-only and a value; copies share the blob.

## Known gaps

- No `fs.Sub` on FS, and no `[]str` or name-and-size form of the directive: a directive goes over a
  `str`, read with `Parse`.
- A single file is a one-record tree (`Parse(site).ReadFile(name)`); there is no separate single-string
  accessor.
- Symbolic links and non-regular files are refused at compile time, not embedded.
- The zero value of FS has no maps; use Parse.
