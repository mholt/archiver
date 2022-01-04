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
)

// FileSystem opens the file at root as a read-only file system. The root may be a
// path to a directory, archive file, compressed archive file or any other file on
// disk.
//
// If root is a directory, its contents are accessed directly from the disk's file system.
// If root is an archive file, its contents can be accessed like a normal directory;
// compressed archive files are transparently decompressed as contents are accessed.
// And if root is any other file, it is the only file in the returned file system.
//
// This method essentially offers uniform read access to various kinds of files:
// directories, archives, compressed archives, and individual files are all treated
// the same way.
func FileSystem(root string) (fs.ReadDirFS, error) {
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
	format, err := Identify(filepath.Base(root), file)
	if err != nil && !errors.Is(err, ErrNoMatch) {
		return nil, err
	}
	if format != nil {
		if af, ok := format.(Archival); ok {
			return ArchiveFS{Path: root, Format: af}, nil
		}
	}

	// otherwise consider it an ordinary file; make a file system with it as its only file
	return FileFS(root), nil
}

// DirFS allows accessing a directory on disk with a consistent file system interface.
// It is almost the same as os.DirFS, except for some reason os.DirFS only implements
// Open() and Stat(), but we also need ReadDir(). Seems like an obvious miss (as of Go 1.17)
// and I have questions: https://twitter.com/mholt6/status/1476058551432876032
type DirFS string

func (f DirFS) Open(name string) (fs.File, error) {
	if err := f.checkName(name, "open"); err != nil {
		return nil, err
	}
	return os.Open(filepath.Join(string(f), name))
}

func (f DirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := f.checkName(name, "readdir"); err != nil {
		return nil, err
	}
	return os.ReadDir(filepath.Join(string(f), name))
}

func (f DirFS) Stat(name string) (fs.FileInfo, error) {
	if err := f.checkName(name, "stat"); err != nil {
		return nil, err
	}
	return os.Stat(filepath.Join(string(f), name))
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
type FileFS string

func (f FileFS) Open(name string) (fs.File, error) {
	if err := f.checkName(name, "open"); err != nil {
		return nil, err
	}
	return os.Open(string(f))
}

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

func (f FileFS) Stat(name string) (fs.FileInfo, error) {
	if err := f.checkName(name, "stat"); err != nil {
		return nil, err
	}
	return os.Stat(string(f))
}

func (f FileFS) checkName(name, op string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if name != "." && name != path.Base(string(f)) {
		return &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
	}
	return nil
}

// ArchiveFS allows accessing an archive (or a compressed archive) using a
// consistent file system interface. Essentially, it allows traversal and
// reading of archive contents the same way as any normal directory on disk.
// The contents of compressed archives are transparently decompressed.
type ArchiveFS struct {
	Path    string
	Format  Archival
	Context context.Context // optional
}

func (f ArchiveFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	archiveFile, err := os.Open(f.Path)
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

	// handle special case of opening the archive root
	if name == "." {
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

	err = f.Format.Extract(f.Context, archiveFile, []string{name}, handler)
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

	return fsFile, nil
}

func (f ArchiveFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	if name == "." {
		fileInfo, err := os.Stat(f.Path)
		if err != nil {
			return nil, err
		}
		return dirFileInfo{fileInfo}, nil
	}

	archiveFile, err := os.Open(f.Path)
	if err != nil {
		return nil, err
	}
	defer archiveFile.Close()

	var result File
	handler := func(_ context.Context, file File) error {
		result = file
		return errStopWalk
	}
	err = f.Format.Extract(f.Context, archiveFile, []string{name}, handler)
	if err != nil && result.FileInfo == nil {
		return nil, err
	}
	if result.FileInfo == nil {
		return nil, fs.ErrNotExist
	}
	return result.FileInfo, err
}

func (f ArchiveFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}

	archiveFile, err := os.Open(f.Path)
	if err != nil {
		return nil, err
	}
	defer archiveFile.Close()

	var entries []fs.DirEntry
	handler := func(_ context.Context, file File) error {
		// directories may end with trailing slash; standardize name
		trimmedName := strings.Trim(file.NameInArchive, "/")

		// don't include the named directory itself in the list of entries
		if trimmedName == name {
			return nil
		}

		entries = append(entries, fs.FileInfoToDirEntry(file))

		// don't traverse into subfolders
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

	err = f.Format.Extract(f.Context, archiveFile, filter, handler)
	return entries, err
}

// errStopWalk is an arbitrary error value, since returning
// any error (other than fs.SkipDir) will stop a walk. We
// use this as we may only want 1 file from an extraction,
// even if that file is a directory and would otherwise be
// traversed during the walk.
var errStopWalk = fmt.Errorf("stop walk")

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
