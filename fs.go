package archiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// FileSystem identifies the format of the input and returns a read-only file system.
// The input can be a filename, stream, or both.
//
// If only a filename is specified, it may be a path to a directory, archive file,
// compressed archive file, compressed regular file, or any other regular file on
// disk. If the filename is a directory, its contents are accessed directly from
// the device's file system. If the filename is an archive file, the contents can
// be accessed like a normal directory; compressed archive files are transparently
// decompressed as contents are accessed. And if the filename is any other file, it
// is the only file in the returned file system; if the file is compressed, it is
// transparently decompressed when read from.
//
// If a stream is specified, the filename (if available) is used as a hint to help
// identify its format. Streams of archive files must be able to be made into an
// io.SectionReader (for safe concurrency) which requires io.ReaderAt and io.Seeker
// (to efficiently determine size). The automatic format identification requires
// io.Reader and will use io.Seeker if supported to avoid buffering.
//
// Whether the data comes from disk or a stream, it is peeked at to automatically
// detect which format to use.
//
// This function essentially offers uniform read access to various kinds of files:
// directories, archives, compressed archives, individual files, and file streams
// are all treated the same way.
//
// NOTE: The performance of compressed tar archives is not great due to overhead
// with decompression. However, the fs.WalkDir() use case has been optimized to
// create an index on first call to ReadDir().
func FileSystem(ctx context.Context, filename string, stream ReaderAtSeeker) (fs.FS, error) {
	if filename == "" && stream == nil {
		return nil, errors.New("no input")
	}

	// if an input stream is specified, we'll use that for identification
	// and for ArchiveFS (if it's an archive); but if not, we'll open the
	// file and read it for identification, but in that case we won't want
	// to also use it for the ArchiveFS (because we need to close what we
	// opened, and ArchiveFS opens its own files), hence this separate var
	idStream := stream

	// if input is only a filename (no stream), check if it's a directory;
	// if not, open it so we can determine which format to use (filename
	// is not always a good indicator of file format)
	if filename != "" && stream == nil {
		info, err := os.Stat(filename)
		if err != nil {
			return nil, err
		}

		// real folders can be accessed easily
		if info.IsDir() {
			return os.DirFS(filename), nil
		}

		// if any archive formats recognize this file, access it like a folder
		file, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		idStream = file // use file for format identification only
	}

	// normally, callers should use the Reader value returned from Identify, but
	// our input is a Seeker, so we know the original input value gets returned
	format, _, err := Identify(ctx, filepath.Base(filename), idStream)
	if errors.Is(err, NoMatch) {
		return FileFS{Path: filename}, nil // must be an ordinary file
	}
	if err != nil {
		return nil, fmt.Errorf("identify format: %w", err)
	}

	switch fileFormat := format.(type) {
	case Extractor:
		// if no stream was input, return an ArchiveFS that relies on the filepath
		if stream == nil {
			return &ArchiveFS{Path: filename, Format: fileFormat, Context: ctx}, nil
		}

		// otherwise, if a stream was input, return an ArchiveFS that relies on that

		// determine size -- we know that the stream value we get back from
		// Identify is the same type as what we input because it is a Seeker
		size, err := streamSizeBySeeking(stream)
		if err != nil {
			return nil, fmt.Errorf("seeking for size: %w", err)
		}

		sr := io.NewSectionReader(stream, 0, size)

		return &ArchiveFS{Stream: sr, Format: fileFormat, Context: ctx}, nil

	case Compression:
		return FileFS{Path: filename, Compression: fileFormat}, nil
	}

	return nil, fmt.Errorf("unable to create file system rooted at %s due to unsupported file or folder type", filename)
}

// ReaderAtSeeker is a type that can read, read at, and seek.
// os.File and io.SectionReader both implement this interface.
type ReaderAtSeeker interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

// FileFS allows accessing a file on disk using a consistent file system interface.
// The value should be the path to a regular file, not a directory. This file will
// be the only entry in the file system and will be at its root. It can be accessed
// within the file system by the name of "." or the filename.
//
// If the file is compressed, set the Compression field so that reads from the
// file will be transparently decompressed.
type FileFS struct {
	// The path to the file on disk.
	Path string

	// If file is compressed, setting this field will
	// transparently decompress reads.
	Compression Decompressor
}

// Open opens the named file, which must be the file used to create the file system.
func (f FileFS) Open(name string) (fs.File, error) {
	if err := f.checkName(name, "open"); err != nil {
		return nil, err
	}
	file, err := os.Open(f.Path)
	if err != nil {
		return nil, err
	}
	if f.Compression == nil {
		return file, nil
	}
	r, err := f.Compression.OpenReader(file)
	if err != nil {
		return nil, err
	}
	return compressedFile{r, closeBoth{file, r}}, nil
}

// Stat stats the named file, which must be the file used to create the file system.
func (f FileFS) Stat(name string) (fs.FileInfo, error) {
	if err := f.checkName(name, "stat"); err != nil {
		return nil, err
	}
	return os.Stat(f.Path)
}

// ReadDir returns a directory listing with the file as the singular entry.
func (f FileFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := f.checkName(name, "stat"); err != nil {
		return nil, err
	}
	info, err := f.Stat(name)
	if err != nil {
		return nil, err
	}
	return []fs.DirEntry{fs.FileInfoToDirEntry(info)}, nil
}

// checkName ensures the name is a valid path and also, in the case of
// the FileFS, that it is either ".", the filename originally passed in
// to create the FileFS, or the base of the filename (name without path).
// Other names do not make sense for a FileFS since the FS is only 1 file.
func (f FileFS) checkName(name, op string) error {
	if name == f.Path {
		return nil
	}
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	if name != "." && name != filepath.Base(f.Path) {
		return &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
	}
	return nil
}

// compressedFile is an fs.File that specially reads
// from a decompression reader, and which closes both
// that reader and the underlying file.
type compressedFile struct {
	io.Reader // decompressor
	closeBoth // file and decompressor
}

// ArchiveFS allows reading an archive (or a compressed archive) using a
// consistent file system interface. Essentially, it allows traversal and
// reading of archive contents the same way as any normal directory on disk.
// The contents of compressed archives are transparently decompressed.
//
// A valid ArchiveFS value must set either Path or Stream, but not both.
// If Path is set, a literal file will be opened from the disk.
// If Stream is set, new SectionReaders will be implicitly created to
// access the stream, enabling safe, concurrent access.
//
// NOTE: Due to Go's file system APIs (see package io/fs), the performance
// of ArchiveFS can suffer when using fs.WalkDir(). To mitigate this,
// an optimized fs.ReadDirFS has been implemented that indexes the entire
// archive on the first call to ReadDir() (since the entire archive needs
// to be walked for every call to ReadDir() anyway, as archive contents are
// often unordered). The first call to ReadDir(), i.e. near the start of the
// walk, will be slow for large archives, but should be instantaneous after.
// If you don't care about walking a file system in directory order, consider
// calling Extract() on the underlying archive format type directly, which
// walks the archive in entry order, without needing to do any sorting.
//
// Note that fs.FS implementations, including this one, reject paths starting
// with "./". This can be problematic sometimes, as it is not uncommon for
// tarballs to contain a top-level/root directory literally named ".", which
// can happen if a tarball is created in the same directory it is archiving.
// The underlying Extract() calls are faithful to entries with this name,
// but file systems have certain semantics around "." that restrict its use.
// For example, a file named "." cannot be created on a real file system
// because it is a special name that means "current directory".
//
// We had to decide whether to honor the true name in the archive, or honor
// file system semantics. Given that this is a virtual file system and other
// code using the fs.FS APIs will trip over a literal directory named ".",
// we choose to honor file system semantics. Files named "." are ignored;
// directories with this name are effectively transparent; their contents
// get promoted up a directory/level. This means a file at "./x" where "."
// is a literal directory name, its name will be passed in as "x" in
// WalkDir callbacks. If you need the raw, uninterpeted values from an
// archive, use the formats' Extract() method directly. See
// https://github.com/golang/go/issues/70155 for a little more background.
//
// This does have one negative edge case... a tar containing contents like
// [x . ./x] will have a conflict on the file named "x" because "./x" will
// also be accessed with the name of "x".
type ArchiveFS struct {
	// set one of these
	Path   string            // path to the archive file on disk, or...
	Stream *io.SectionReader // ...stream from which to read archive

	Format  Extractor       // the archive format
	Prefix  string          // optional subdirectory in which to root the fs
	Context context.Context // optional; mainly for cancellation

	// amortizing cache speeds up walks (esp. ReadDir)
	contents map[string]fs.FileInfo
	dirs     map[string][]fs.DirEntry
}

// context always return a context, preferring f.Context if not nil.
func (f ArchiveFS) context() context.Context {
	if f.Context != nil {
		return f.Context
	}
	return context.Background()
}

// Open opens the named file from within the archive. If name is "." then
// the archive file itself will be opened as a directory file.
func (f ArchiveFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("%w: %s", fs.ErrInvalid, name)}
	}

	// apply prefix if fs is rooted in a subtree
	name = path.Join(f.Prefix, name)

	// if we've already indexed the archive, we can know quickly if the file doesn't exist,
	// and we can also return directory files with their entries instantly
	if f.contents != nil {
		if info, found := f.contents[name]; found {
			if info.IsDir() {
				if entries, ok := f.dirs[name]; ok {
					return &dirFile{info: info, entries: entries}, nil
				}
			}
		} else {
			if entries, found := f.dirs[name]; found {
				return &dirFile{info: implicitDirInfo{implicitDirEntry{name}}, entries: entries}, nil
			}
			return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("open %s: %w", name, fs.ErrNotExist)}
		}
	}

	// if a filename is specified, open the archive file
	var archiveFile *os.File
	var err error
	if f.Stream == nil {
		archiveFile, err = os.Open(f.Path)
		if err != nil {
			return nil, err
		}
		defer func() {
			// close the archive file if extraction failed; we can only
			// count on the user/caller closing it if they successfully
			// got the handle to the extracted file
			if err != nil {
				archiveFile.Close()
			}
		}()
	} else if f.Stream == nil {
		return nil, fmt.Errorf("no input; one of Path or Stream must be set")
	}

	// handle special case of opening the archive root
	if name == "." {
		var archiveInfo fs.FileInfo
		if archiveFile != nil {
			archiveInfo, err = archiveFile.Stat()
			if err != nil {
				return nil, err
			}
		} else {
			archiveInfo = implicitDirInfo{
				implicitDirEntry{"."},
			}
		}
		var entries []fs.DirEntry
		entries, err = f.ReadDir(name)
		if err != nil {
			return nil, err
		}
		if err := archiveFile.Close(); err != nil {
			return nil, err
		}
		return &dirFile{
			info:    dirFileInfo{archiveInfo},
			entries: entries,
		}, nil
	}

	var inputStream io.Reader
	if f.Stream == nil {
		inputStream = archiveFile
	} else {
		inputStream = io.NewSectionReader(f.Stream, 0, f.Stream.Size())
	}

	var decompressor io.ReadCloser
	if decomp, ok := f.Format.(Decompressor); ok {
		decompressor, err = decomp.OpenReader(inputStream)
		if err != nil {
			return nil, err
		}
		inputStream = decompressor
	}

	// prepare the handler that we'll need if we have to iterate the
	// archive to find the file being requested
	var fsFile fs.File
	handler := func(ctx context.Context, file FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		// paths in archives can't necessarily be trusted; also clean up any "./" prefix
		file.NameInArchive = path.Clean(file.NameInArchive)

		if !strings.HasPrefix(file.NameInArchive, name) {
			return nil
		}

		// if this is the requested file, and it's a directory, set up the dirFile,
		// which will include a listing of all its contents as we continue the walk
		if file.NameInArchive == name && file.IsDir() {
			fsFile = &dirFile{info: file} // will fill entries slice as we continue the walk
			return nil
		}

		// if the named file was a directory and we are filling its entries,
		// add this entry to the list
		if df, ok := fsFile.(*dirFile); ok {
			df.entries = append(df.entries, fs.FileInfoToDirEntry(file))

			// don't traverse into subfolders
			if file.IsDir() {
				return fs.SkipDir
			}

			return nil
		}

		innerFile, err := file.Open()
		if err != nil {
			return err
		}

		fsFile = closeBoth{File: innerFile, c: archiveFile}

		if decompressor != nil {
			fsFile = closeBoth{fsFile, decompressor}
		}

		return fs.SkipAll
	}

	// when we start the walk, we pass in a nil list of files to extract, since
	// files may have a "." component in them, and the underlying format doesn't
	// know about our file system semantics, so we need to filter ourselves (it's
	// not significantly less efficient).
	if ar, ok := f.Format.(Archive); ok {
		// bypass the CompressedArchive format's opening of the decompressor, since
		// we already did it because we need to keep it open after returning.
		// "I BYPASSED THE COMPRESSOR!" -Rey
		err = ar.Extraction.Extract(f.context(), inputStream, handler)
	} else {
		err = f.Format.Extract(f.context(), inputStream, handler)
	}
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("extract: %w", err)}
	}
	if fsFile == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("open %s: %w", name, fs.ErrNotExist)}
	}

	return fsFile, nil
}

// Stat stats the named file from within the archive. If name is "." then
// the archive file itself is statted and treated as a directory file.
func (f ArchiveFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fmt.Errorf("%s: %w", name, fs.ErrInvalid)}
	}

	if name == "." {
		if f.Path != "" {
			fileInfo, err := os.Stat(f.Path)
			if err != nil {
				return nil, err
			}
			return dirFileInfo{fileInfo}, nil
		} else if f.Stream != nil {
			return implicitDirInfo{implicitDirEntry{name}}, nil
		}
	}

	// apply prefix if fs is rooted in a subtree
	name = path.Join(f.Prefix, name)

	// if archive has already been indexed, simply use it
	if f.contents != nil {
		if info, ok := f.contents[name]; ok {
			return info, nil
		}
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fmt.Errorf("stat %s: %w", name, fs.ErrNotExist)}
	}

	var archiveFile *os.File
	var err error
	if f.Stream == nil {
		archiveFile, err = os.Open(f.Path)
		if err != nil {
			return nil, err
		}
		defer archiveFile.Close()
	}

	var result FileInfo
	handler := func(ctx context.Context, file FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if path.Clean(file.NameInArchive) == name {
			result = file
			return fs.SkipAll
		}
		return nil
	}
	var inputStream io.Reader = archiveFile
	if f.Stream != nil {
		inputStream = io.NewSectionReader(f.Stream, 0, f.Stream.Size())
	}
	err = f.Format.Extract(f.context(), inputStream, handler)
	if err != nil && result.FileInfo == nil {
		return nil, err
	}
	if result.FileInfo == nil {
		return nil, fs.ErrNotExist
	}
	return result.FileInfo, nil
}

// ReadDir reads the named directory from within the archive. If name is "."
// then the root of the archive content is listed.
func (f *ArchiveFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}

	// apply prefix if fs is rooted in a subtree
	name = path.Join(f.Prefix, name)

	// fs.WalkDir() calls ReadDir() once per directory, and for archives with
	// lots of directories, that is very slow, since we have to traverse the
	// entire archive in order to ensure that we got all the entries for a
	// directory -- so we can fast-track this lookup if we've done the
	// traversal already
	if len(f.dirs) > 0 {
		return f.dirs[name], nil
	}

	f.contents = make(map[string]fs.FileInfo)
	f.dirs = make(map[string][]fs.DirEntry)

	var archiveFile *os.File
	var err error
	if f.Stream == nil {
		archiveFile, err = os.Open(f.Path)
		if err != nil {
			return nil, err
		}
		defer archiveFile.Close()
	}

	handler := func(ctx context.Context, file FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		// can't always trust path names
		file.NameInArchive = path.Clean(file.NameInArchive)

		// avoid infinite walk; apparently, creating a tar file in the target
		// directory may result in an entry called "." in the archive; see #384
		if file.NameInArchive == "." {
			return nil
		}

		// if the name being requested isn't a directory, return an error similar to
		// what most OSes return from the readdir system call when given a non-dir
		if file.NameInArchive == name && !file.IsDir() {
			return &fs.PathError{Op: "readdir", Path: name, Err: errors.New("not a directory")}
		}

		// index this file info for quick access
		f.contents[file.NameInArchive] = file

		// amortize the DirEntry list per directory, and prefer the real entry's DirEntry over an implicit/fake
		// one we may have created earlier; first try to find if it exists, and if so, replace the value;
		// otherwise insert it in sorted position
		dir := path.Dir(file.NameInArchive)
		dirEntry := fs.FileInfoToDirEntry(file)
		idx, found := slices.BinarySearchFunc(f.dirs[dir], dirEntry, func(a, b fs.DirEntry) int {
			return strings.Compare(a.Name(), b.Name())
		})
		if found {
			f.dirs[dir][idx] = dirEntry
		} else {
			f.dirs[dir] = slices.Insert(f.dirs[dir], idx, dirEntry)
		}

		// this loop looks like an abomination, but it's really quite simple: we're
		// just iterating the directories of the path up to the root; i.e. we lob off
		// the base (last component) of the path until no separators remain, i.e. only
		// one component remains -- then loop again to make sure it's not a duplicate
		// (start without the base, since we know the full filename is an actual entry
		// in the archive, we don't need to create an implicit directory entry for it)
		startingPath := path.Dir(file.NameInArchive)
		for dir, base := path.Dir(startingPath), path.Base(startingPath); base != "."; dir, base = path.Dir(dir), path.Base(dir) {
			if err := ctx.Err(); err != nil {
				return err
			}

			var dirInfo fs.DirEntry = implicitDirInfo{implicitDirEntry{base}}

			// we are "filling in" any directories that could potentially be only implicit,
			// and since a nested directory can have more than 1 item, we need to prevent
			// duplication; for example: given a/b/c and a/b/d, we need to avoid adding
			// an entry for "b" twice within "a" -- hence we search for it first, and if
			// it doesn't already exist, we insert it in sorted position
			idx, found := slices.BinarySearchFunc(f.dirs[dir], dirInfo, func(a, b fs.DirEntry) int {
				return strings.Compare(a.Name(), b.Name())
			})
			if !found {
				f.dirs[dir] = slices.Insert(f.dirs[dir], idx, dirInfo)
			}
		}

		return nil
	}

	var inputStream io.Reader = archiveFile
	if f.Stream != nil {
		inputStream = io.NewSectionReader(f.Stream, 0, f.Stream.Size())
	}

	err = f.Format.Extract(f.context(), inputStream, handler)
	if err != nil {
		// these being non-nil implies that we have indexed the archive,
		// but if an error occurred, we likely only got part of the way
		// through and our index is incomplete, and we'd have to re-walk
		// the whole thing anyway; so reset these to nil to avoid bugs
		f.dirs = nil
		f.contents = nil
		return nil, fmt.Errorf("extract: %w", err)
	}

	return f.dirs[name], nil
}

// Sub returns an FS corresponding to the subtree rooted at dir.
func (f *ArchiveFS) Sub(dir string) (fs.FS, error) {
	if !fs.ValidPath(dir) {
		return nil, &fs.PathError{Op: "sub", Path: dir, Err: fs.ErrInvalid}
	}
	info, err := f.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	// result is the same as what we're starting with, except
	// we indicate a path prefix to be used for all operations;
	// the reason we don't append to the Path field directly
	// is because the input might be a stream rather than a
	// path on disk, and the Prefix field is applied on both
	result := f
	result.Prefix = dir
	return result, nil
}

// TopDirOpen is a special Open() function that may be useful if
// a file system root was created by extracting an archive.
//
// It first tries the file name as given, but if that returns an
// error, it tries the name without the first element of the path.
// In other words, if "a/b/c" returns an error, then "b/c" will
// be tried instead.
//
// Consider an archive that contains a file "a/b/c". When the
// archive is extracted, the contents may be created without a
// new parent/root folder to contain them, and the path of the
// same file outside the archive may be lacking an exclusive root
// or parent container. Thus it is likely for a file system
// created for the same files extracted to disk to be rooted at
// one of the top-level files/folders from the archive instead of
// a parent folder. For example, the file known as "a/b/c" when
// rooted at the archive becomes "b/c" after extraction when rooted
// at "a" on disk (because no new, exclusive top-level folder was
// created). This difference in paths can make it difficult to use
// archives and directories uniformly. Hence these TopDir* functions
// which attempt to smooth over the difference.
//
// Some extraction utilities do create a container folder for
// archive contents when extracting, in which case the user
// may give that path as the root. In that case, these TopDir*
// functions are not necessary (but aren't harmful either). They
// are primarily useful if you are not sure whether the root is
// an archive file or is an extracted archive file, as they will
// work with the same filename/path inputs regardless of the
// presence of a top-level directory.
func TopDirOpen(fsys fs.FS, name string) (fs.File, error) {
	file, err := fsys.Open(name)
	if err == nil {
		return file, nil
	}
	return fsys.Open(pathWithoutTopDir(name))
}

// TopDirStat is like TopDirOpen but for Stat.
func TopDirStat(fsys fs.FS, name string) (fs.FileInfo, error) {
	info, err := fs.Stat(fsys, name)
	if err == nil {
		return info, nil
	}
	return fs.Stat(fsys, pathWithoutTopDir(name))
}

// TopDirReadDir is like TopDirOpen but for ReadDir.
func TopDirReadDir(fsys fs.FS, name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(fsys, name)
	if err == nil {
		return entries, nil
	}
	return fs.ReadDir(fsys, pathWithoutTopDir(name))
}

func pathWithoutTopDir(fpath string) string {
	slashIdx := strings.Index(fpath, "/")
	if slashIdx < 0 {
		return fpath
	}
	return fpath[slashIdx+1:]
}

// dirFile implements the fs.ReadDirFile interface.
type dirFile struct {
	info        fs.FileInfo
	entries     []fs.DirEntry
	entriesRead int // used for paging with ReadDir(n)
}

func (dirFile) Read([]byte) (int, error)      { return 0, errors.New("cannot read a directory file") }
func (df dirFile) Stat() (fs.FileInfo, error) { return df.info, nil }
func (dirFile) Close() error                  { return nil }

// ReadDir implements [fs.ReadDirFile].
func (df *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		return df.entries, nil
	}
	if df.entriesRead >= len(df.entries) {
		return nil, io.EOF
	}
	if df.entriesRead+n > len(df.entries) {
		n = len(df.entries) - df.entriesRead
	}
	entries := df.entries[df.entriesRead : df.entriesRead+n]
	df.entriesRead += n
	return entries, nil
}

// dirFileInfo is an implementation of fs.FileInfo that
// is only used for files that are directories. It always
// returns 0 size, directory bit set in the mode, and
// true for IsDir. It is often used as the FileInfo for
// dirFile values.
type dirFileInfo struct {
	fs.FileInfo
}

func (dirFileInfo) Size() int64            { return 0 }
func (info dirFileInfo) Mode() fs.FileMode { return info.FileInfo.Mode() | fs.ModeDir }
func (dirFileInfo) IsDir() bool            { return true }

// fileInArchive represents a file that is opened from within an archive.
// It implements fs.File.
type fileInArchive struct {
	io.ReadCloser
	info fs.FileInfo
}

func (af fileInArchive) Stat() (fs.FileInfo, error) { return af.info, nil }

// closeBoth closes both the file and an associated
// closer, such as a (de)compressor that wraps the
// reading/writing of the file. See issue #365. If a
// better solution is found, I'd probably prefer that.
type closeBoth struct {
	fs.File
	c io.Closer // usually the archive or the decompressor
}

// Close closes both the file and the associated closer. It always calls
// Close() on both, but if multiple errors occur they are wrapped together.
func (dc closeBoth) Close() error {
	var err error
	if dc.File != nil {
		if err2 := dc.File.Close(); err2 != nil {
			err = fmt.Errorf("closing file: %w", err2)
		}
	}
	if dc.c != nil {
		if err2 := dc.c.Close(); err2 != nil {
			if err == nil {
				err = fmt.Errorf("closing closer: %w", err2)
			} else {
				err = fmt.Errorf("%w; additionally, closing closer: %w", err, err2)
			}
		}
	}
	return err
}

// implicitDirEntry represents a directory that does
// not actually exist in the archive but is inferred
// from the paths of actual files in the archive.
type implicitDirEntry struct{ name string }

func (e implicitDirEntry) Name() string    { return e.name }
func (implicitDirEntry) IsDir() bool       { return true }
func (implicitDirEntry) Type() fs.FileMode { return fs.ModeDir }
func (e implicitDirEntry) Info() (fs.FileInfo, error) {
	return implicitDirInfo{e}, nil
}

// implicitDirInfo is a fs.FileInfo for an implicit directory
// (implicitDirEntry) value. This is used when an archive may
// not contain actual entries for a directory, but we need to
// pretend it exists so its contents can be discovered and
// traversed.
type implicitDirInfo struct{ implicitDirEntry }

func (d implicitDirInfo) Name() string      { return d.name }
func (implicitDirInfo) Size() int64         { return 0 }
func (d implicitDirInfo) Mode() fs.FileMode { return d.Type() }
func (implicitDirInfo) ModTime() time.Time  { return time.Time{} }
func (implicitDirInfo) Sys() any            { return nil }

// Interface guards
var (
	_ fs.ReadDirFS = (*FileFS)(nil)
	_ fs.StatFS    = (*FileFS)(nil)

	_ fs.ReadDirFS = (*ArchiveFS)(nil)
	_ fs.StatFS    = (*ArchiveFS)(nil)
	_ fs.SubFS     = (*ArchiveFS)(nil)
)
