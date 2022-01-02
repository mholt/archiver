package archiver

import (
	"context"
	"errors"
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
// If root is a directory, its contents are accessed directly like normal.
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

	//otherwise consider it an ordinary file; make a file system with it as its only file
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
// within the file system only by using empty string, ".", or the filename.
type FileFS string

func (f FileFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if err := f.checkName(name, "open"); err != nil {
		return nil, err
	}
	return os.Open(string(f))
}

func (f FileFS) ReadDir(name string) ([]fs.DirEntry, error) {
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
	if name != "" && name != "." && name != path.Base(string(f)) {
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

	var fsFile fs.File
	handler := func(_ context.Context, file File) error {
		rc, err := file.Open()
		if err != nil {
			return err
		}
		fsFile = extractedFile{File: file, ReadCloser: rc, parentArchive: archiveFile}
		return nil
	}

	err = f.Format.Extract(f.Context, archiveFile, []string{name}, handler)
	return fsFile, err
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
		entries = append(entries, fs.FileInfoToDirEntry(file))
		return nil
	}

	err = f.Format.Extract(f.Context, archiveFile, []string{name}, handler)
	return entries, err
}

func (f ArchiveFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	archiveFile, err := os.Open(f.Path)
	if err != nil {
		return nil, err
	}
	defer archiveFile.Close()

	var result File
	handler := func(_ context.Context, file File) error {
		result = file
		return nil
	}
	err = f.Format.Extract(f.Context, archiveFile, []string{name}, handler)
	return result, err
}

// extractedFile ensures that the parent archive is closed
// when the file from within it is also closed.
type extractedFile struct {
	File
	io.ReadCloser // the file's reader
	parentArchive io.Closer
}

func (ef extractedFile) Close() error {
	if err := ef.parentArchive.Close(); err != nil {
		return err
	}
	return ef.ReadCloser.Close()
}
