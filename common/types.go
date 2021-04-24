package common

import (
	"io"
	"os"
)

// File provides methods for accessing information about
// or contents of a file within an archive.
type File struct {
	os.FileInfo

	// The original header info; depends on
	// type of archive -- could be nil, too.
	Header interface{}

	// Allow the file contents to be read (and closed)
	io.ReadCloser
}

// FileInfo is an os.FileInfo but optionally with
// a custom name, useful if dealing with files that
// are not actual files on disk, or which have a
// different name in an archive than on disk.
type FileInfo struct {
	os.FileInfo
	CustomName string
}

// Name returns fi.CustomName if not empty;
// otherwise it returns fi.FileInfo.Name().
func (fi FileInfo) Name() string {
	if fi.CustomName != "" {
		return fi.CustomName
	}
	return fi.FileInfo.Name()
}
