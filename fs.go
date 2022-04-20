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
	"runtime"
	"strings"
	"time"
)

// FileSystem opens the file at root as a read-only file system. The root may be a
// path to a directory, archive file, compressed archive file, compressed file, or
// any other file on disk.
//
// If root is a directory, its contents are accessed directly from the disk's file system.
// If root is an archive file, its contents can be accessed like a normal directory;
// compressed archive files are transparently decompressed as contents are accessed.
// And if root is any other file, it is the only file in the file system; if the file
// is compressed, it is transparently decompressed when read from.
//
// This method essentially offers uniform read access to various kinds of files:
// directories, archives, compressed archives, and individual files are all treated
// the same way.
//
// The returned FS values are guaranteed to be fs.ReadDirFS and fs.StatFS types, and
// may also be fs.SubFS.
func FileSystem(root string) (fs.FS, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	// real folders can be accessed easily
	if info.IsDir() {
		return DirFS(root), nil
	}

	// if any archive formats recognize this file, access it like a folder
	file, err := os.Open(root)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	format, _, err := Identify(filepath.Base(root), file)
	if err != nil && !errors.Is(err, ErrNoMatch) {
		return nil, err
	}
	if format != nil {
		// TODO: we only really need Extractor and Decompressor here, not the combined interfaces...
		if af, ok := format.(Archival); ok {
			return ArchiveFS{Path: root, Format: af}, nil
		}
		if cf, ok := format.(Compression); ok {
			return FileFS{Path: root, Compression: cf}, nil
		}
	}

	// otherwise consider it an ordinary file; make a file system with it as its only file
	return FileFS{Path: root}, nil
}

// DirFS allows accessing a directory on disk with a consistent file system interface.
// It is almost the same as os.DirFS, except for some reason os.DirFS only implements
// Open() and Stat(), but we also need ReadDir(). Seems like an obvious miss (as of Go 1.17)
// and I have questions: https://twitter.com/mholt6/status/1476058551432876032
type DirFS string

// Open opens the named file.
func (f DirFS) Open(name string) (fs.File, error) {
	if err := f.checkName(name, "open"); err != nil {
		return nil, err
	}
	return os.Open(filepath.Join(string(f), name))
}

// ReadDir returns a listing of all the files in the named directory.
func (f DirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := f.checkName(name, "readdir"); err != nil {
		return nil, err
	}
	return os.ReadDir(filepath.Join(string(f), name))
}

// Stat returns info about the named file.
func (f DirFS) Stat(name string) (fs.FileInfo, error) {
	if err := f.checkName(name, "stat"); err != nil {
		return nil, err
	}
	return os.Stat(filepath.Join(string(f), name))
}

// Sub returns an FS corresponding to the subtree rooted at dir.
func (f DirFS) Sub(dir string) (fs.FS, error) {
	if err := f.checkName(dir, "sub"); err != nil {
		return nil, err
	}
	info, err := f.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	return DirFS(filepath.Join(string(f), dir)), nil
}

// checkName returns an error if name is not a valid path according to the docs of
// the io/fs package, with an extra cue taken from the standard lib's implementation
// of os.dirFS.Open(), which checks for invalid characters in Windows paths.
func (f DirFS) checkName(name, op string) error {
	if !fs.ValidPath(name) || runtime.GOOS == "windows" && strings.ContainsAny(name, `\:`) {
		return &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	return nil
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
	return compressedFile{file, r}, nil
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

// Stat stats the named file, which must be the file used to create the file system.
func (f FileFS) Stat(name string) (fs.FileInfo, error) {
	if err := f.checkName(name, "stat"); err != nil {
		return nil, err
	}
	return os.Stat(f.Path)
}

func (f FileFS) checkName(name, op string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if name != "." && name != path.Base(f.Path) {
		return &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
	}
	return nil
}

// compressedFile is an fs.File that specially reads
// from a decompression reader, and which closes both
// that reader and the underlying file.
type compressedFile struct {
	*os.File
	decomp io.ReadCloser
}

func (cf compressedFile) Read(p []byte) (int, error) { return cf.decomp.Read(p) }
func (cf compressedFile) Close() error {
	err := cf.File.Close()
	err2 := cf.decomp.Close()
	if err2 != nil && err == nil {
		err = err2
	}
	return err
}

// ArchiveFS allows accessing an archive (or a compressed archive) using a
// consistent file system interface. Essentially, it allows traversal and
// reading of archive contents the same way as any normal directory on disk.
// The contents of compressed archives are transparently decompressed.
//
// A valid ArchiveFS value must set either Path or Stream. If Path is set,
// a literal file will be opened from the disk. If Stream is set, new
// SectionReaders will be implicitly created to access the stream, enabling
// safe, concurrent access.
//
// NOTE: Due to Go's file system APIs (see package io/fs), the performance
// of ArchiveFS when used with fs.WalkDir() is poor for archives with lots
// of files (see issue #326). The fs.WalkDir() API requires listing each
// directory's contents in turn, and the only way to ensure we return the
// complete list of folder contents is to traverse the whole archive and
// build a slice; so if this is done for the root of an archive with many
// files, performance tends toward O(n^2) as the entire archive is walked
// for every folder that is enumerated (WalkDir calls ReadDir recursively).
// If you do not need each directory's contents walked in order, please
// prefer calling Extract() from an archive type directly; this will perform
// a O(n) walk of the contents in archive order, rather than the slower
// directory tree order.
type ArchiveFS struct {
	// set one of these
	Path   string            // path to the archive file on disk, or...
	Stream *io.SectionReader // ...stream from which to read archive

	Format  Archival        // the archive format
	Prefix  string          // optional subdirectory in which to root the fs
	Context context.Context // optional
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
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	var archiveFile fs.File
	var err error
	if f.Path != "" {
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
	} else if f.Stream != nil {
		archiveFile = fakeArchiveFile{}
	}

	// apply prefix if fs is rooted in a subtree
	name = path.Join(f.Prefix, name)

	// handle special case of opening the archive root
	if name == "." && archiveFile != nil {
		archiveInfo, err := archiveFile.Stat()
		if err != nil {
			return nil, err
		}
		entries, err := f.ReadDir(name)
		if err != nil {
			return nil, err
		}
		return &dirFile{
			extractedFile: extractedFile{
				File: File{
					FileInfo:      dirFileInfo{archiveInfo},
					NameInArchive: ".",
				},
			},
			entries: entries,
		}, nil
	}

	var fsFile fs.File
	handler := func(_ context.Context, file File) error {
		// if this is the requested file, and it's a directory, set up the dirFile,
		// which will include a listing of all its contents as we continue the walk
		trimmedName := strings.Trim(file.NameInArchive, "/")
		if trimmedName == name && file.IsDir() {
			fsFile = &dirFile{extractedFile: extractedFile{File: file}}
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

		// if named file is not a regular file, it can't be opened
		if !file.Mode().IsRegular() {
			fsFile = extractedFile{File: file}
			return errStopWalk
		}

		// regular files can be read, so open it for reading
		rc, err := file.Open()
		if err != nil {
			return err
		}
		fsFile = extractedFile{File: file, ReadCloser: rc, parentArchive: archiveFile}
		return errStopWalk
	}

	var inputStream io.Reader = archiveFile
	if f.Stream != nil {
		inputStream = io.NewSectionReader(f.Stream, 0, f.Stream.Size())
	}

	err = f.Format.Extract(f.context(), inputStream, []string{name}, handler)
	if err != nil && fsFile != nil {
		if ef, ok := fsFile.(extractedFile); ok {
			if ef.parentArchive != nil {
				// don't close the archive file in above defer; it
				// will be closed when the returned file is closed
				err = nil
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if fsFile == nil {
		return nil, fs.ErrNotExist
	}

	return fsFile, nil
}

// Stat stats the named file from within the archive. If name is "." then
// the archive file itself is statted and treated as a directory file.
func (f ArchiveFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	// apply prefix if fs is rooted in a subtree
	name = path.Join(f.Prefix, name)

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

	var archiveFile *os.File
	var err error
	if f.Stream == nil {
		archiveFile, err = os.Open(f.Path)
		if err != nil {
			return nil, err
		}
		defer archiveFile.Close()
	}

	var result File
	handler := func(_ context.Context, file File) error {
		// in theory, the first file handled should be the one requested,
		// unless... the file requested is a directory and the archive was
		// created depth-first (i.e. directory contents added before the
		// directory itself), in which case we have to iterate through the
		// contents first; hence the check for exact filename match (issue #310)
		if strings.TrimRight(file.NameInArchive, "/") == strings.TrimRight(name, "/") {
			result = file
			return errStopWalk
		}
		return nil
	}
	var inputStream io.Reader = archiveFile
	if f.Stream != nil {
		inputStream = io.NewSectionReader(f.Stream, 0, f.Stream.Size())
	}
	err = f.Format.Extract(f.context(), inputStream, []string{name}, handler)
	if err != nil && result.FileInfo == nil {
		return nil, err
	}
	if result.FileInfo == nil {
		return nil, fs.ErrNotExist
	}
	return result.FileInfo, nil
}

// ReadDir reads the named directory from within the archive.
func (f ArchiveFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
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

	// apply prefix if fs is rooted in a subtree
	name = path.Join(f.Prefix, name)

	// store entries in a map to inexpensively avoid duplication
	entries := make(map[string]fs.DirEntry)
	handler := func(_ context.Context, file File) error {
		// directories may end with trailing slash; standardize name
		trimmedName := strings.Trim(file.NameInArchive, "/")

		// don't include the named directory itself in the list of entries
		if trimmedName == name {
			return nil
		}

		// items added to an archive depth-first results in the subfolder entry being
		// added to the archive after all the files within it, meaning we won't have
		// the chance to return SkipDir before traversing into it; so we have to also
		// check if we are within a subfolder deeper than the requested name (because
		// this is a ReadDir function, we do not intend to traverse subfolders) (issue #310)
		// in other words, archive entries can be created out-of-(breadth-first)-order,
		// or even an arbitrary/random order, and we need to make sure we get all entries
		// in just this directory
		if path.Dir(trimmedName) != name {
			// additionally, some archive files don't have actual entries for folders,
			// leaving them to be inferred from the names of files instead (issue #330)
			// so as we traverse deeper, we need to implicitly find subfolders within
			// this current directory and add fake entries to the output
			remainingPath := strings.TrimPrefix(file.NameInArchive, name)
			nextDir := topDir(remainingPath)        // if name in archive is "a/b/c" and root is "a", this becomes "b" (the implied folder to add)
			implicitDir := path.Join(name, nextDir) // the full path of the implied directory

			// create fake entry only if no entry currently exists (don't overwrite a real entry)
			if _, ok := entries[implicitDir]; !ok {
				entries[implicitDir] = implicitDirEntry{implicitDir}
			}

			return fs.SkipDir
		}

		entries[file.NameInArchive] = fs.FileInfoToDirEntry(file)

		// don't traverse deeper into subfolders
		if file.IsDir() {
			return fs.SkipDir
		}

		return nil
	}

	// handle special case of reading from root of archive
	var filter []string
	if name != "." {
		filter = []string{name}
	}

	var inputStream io.Reader = archiveFile
	if f.Stream != nil {
		inputStream = io.NewSectionReader(f.Stream, 0, f.Stream.Size())
	}

	err = f.Format.Extract(f.context(), inputStream, filter, handler)
	if err != nil {
		return nil, err
	}

	// convert map to slice
	entriesSlice := make([]fs.DirEntry, 0, len(entries))
	for _, ent := range entries {
		entriesSlice = append(entriesSlice, ent)
	}

	return entriesSlice, nil
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
func TopDirStat(fsys fs.StatFS, name string) (fs.FileInfo, error) {
	info, err := fsys.Stat(name)
	if err == nil {
		return info, nil
	}
	return fsys.Stat(pathWithoutTopDir(name))
}

// TopDirReadDir is like TopDirOpen but for ReadDir.
func TopDirReadDir(fsys fs.ReadDirFS, name string) ([]fs.DirEntry, error) {
	entries, err := fsys.ReadDir(name)
	if err == nil {
		return entries, nil
	}
	return fsys.ReadDir(pathWithoutTopDir(name))
}

func pathWithoutTopDir(fpath string) string {
	slashIdx := strings.Index(fpath, "/")
	if slashIdx < 0 {
		return fpath
	}
	return fpath[slashIdx+1:]
}

// errStopWalk is an arbitrary error value, since returning
// any error (other than fs.SkipDir) will stop a walk. We
// use this as we may only want 1 file from an extraction,
// even if that file is a directory and would otherwise be
// traversed during the walk.
var errStopWalk = fmt.Errorf("stop walk")

type fakeArchiveFile struct{}

func (f fakeArchiveFile) Stat() (fs.FileInfo, error) {
	return implicitDirInfo{
		implicitDirEntry{name: "."},
	}, nil
}
func (f fakeArchiveFile) Read([]byte) (int, error) { return 0, io.EOF }
func (f fakeArchiveFile) Close() error             { return nil }

// dirFile implements the fs.ReadDirFile interface.
type dirFile struct {
	extractedFile

	// TODO: We could probably be more memory-efficient by not loading
	// all the entries at once and then "faking" the paging for ReadDir().
	// Instead, we could maybe store a reference to the parent archive FS,
	// then walk it each time ReadDir is called, skipping entriesRead
	// files, then continuing the listing, until n are listed. But that
	// might be kinda messy and a lot of work, so I leave it for a future
	// optimization if needed.
	entries     []fs.DirEntry
	entriesRead int
}

// If this represents the root of the archive, we use the archive's
// FileInfo which says it's a file, not a directory; the whole point
// of this package is to treat the archive as a directory, so always
// return true in our case.
func (dirFile) IsDir() bool { return true }

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

// extractedFile implements fs.File, thus it represents an "opened" file,
// which is slightly different from our File type which represents a file
// that possibly may be opened. If the file is actually opened, this type
// ensures that the parent archive is closed when this file from within it
// is also closed.
type extractedFile struct {
	File

	// Set these fields if a "regular file" which has actual content
	// that can be read, i.e. a file that is open for reading.
	// ReadCloser should be the file's reader, and parentArchive is
	// a reference to the archive the files comes out of.
	// If parentArchive is set, it will also be closed along with
	// the file when Close() is called.
	io.ReadCloser
	parentArchive io.Closer
}

// Close closes the the current file if opened and
// the parent archive if specified. This is a no-op
// for directories which do not set those fields.
func (ef extractedFile) Close() error {
	if ef.parentArchive != nil {
		if err := ef.parentArchive.Close(); err != nil {
			return err
		}
	}
	if ef.ReadCloser != nil {
		return ef.ReadCloser.Close()
	}
	return nil
}

// implicitDirEntry represents a directory that does
// not actually exist in the archive but is inferred
// from the paths of actual files in the archive.
type implicitDirEntry struct {
	name string
}

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
type implicitDirInfo struct {
	implicitDirEntry
}

func (d implicitDirInfo) Name() string      { return d.name }
func (implicitDirInfo) Size() int64         { return 0 }
func (d implicitDirInfo) Mode() fs.FileMode { return d.Type() }
func (implicitDirInfo) ModTime() time.Time  { return time.Time{} }
func (implicitDirInfo) Sys() interface{}    { return nil }

// Interface guards
var (
	_ fs.ReadDirFS = (*DirFS)(nil)
	_ fs.StatFS    = (*DirFS)(nil)
	_ fs.SubFS     = (*DirFS)(nil)

	_ fs.ReadDirFS = (*FileFS)(nil)
	_ fs.StatFS    = (*FileFS)(nil)

	_ fs.ReadDirFS = (*ArchiveFS)(nil)
	_ fs.StatFS    = (*ArchiveFS)(nil)
	_ fs.SubFS     = (*ArchiveFS)(nil)
)
