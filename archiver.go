// Package archiver facilitates convenient, cross-platform, high-level archival
// and compression operations for a variety of formats and compression algorithms.
//
// This package and its dependencies are written in pure Go (not cgo) and
// have no external dependencies, so they should run on all major platforms.
// (It also comes with a command for CLI use in the cmd/arc folder.)
//
// Each supported format or algorithm has a unique type definition that
// implements the interfaces corresponding to the tasks they perform. For
// example, the Tar type implements Reader, Writer, Archiver, Unarchiver,
// Walker, and several other interfaces.
//
// The most common functions are implemented at the package level for
// convenience: Archive, Unarchive, Walk, Extract, CompressFile, and
// DecompressFile. With these, the format type is chosen implicitly,
// and a sane default configuration is used.
//
// To customize a format's configuration, create an instance of its struct
// with its fields set to the desired values. You can also use and customize
// the handy Default* (replace the wildcard with the format's type name)
// for a quick, one-off instance of the format's type.
//
// To obtain a new instance of a format's struct with the default config, use
// the provided New*() functions. This is not required, however. An empty
// struct of any type, for example &Zip{} is perfectly valid, so you may
// create the structs manually, too. The examples on this page show how
// either may be done.
//
// See the examples in this package for an idea of how to wield this package
// for common tasks. Most of the examples which are specific to a certain
// format type, for example Zip, can be applied to other types that implement
// the same interfaces. For example, using Zip is very similar to using Tar
// or TarGz (etc), and using Gz is very similar to using Sz or Xz (etc).
//
// When creating archives or compressing files using a specific instance of
// the format's type, the name of the output file MUST match that of the
// format, to prevent confusion later on. If you absolutely need a different
// file extension, you may rename the file afterward.
//
// Values in this package are NOT safe for concurrent use. There is no
// performance benefit of reusing them, and since they may contain important
// state (especially while walking, reading, or writing), it is NOT
// recommended to reuse values from this package or change their configuration
// after they are in use.
package archiver

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mholt/archiver/v3/common"
)

// Archiver is a type that can create an archive file
// from a list of source file names.
type Archiver interface {
	ExtensionChecker

	// Archive adds all the files or folders in sources
	// to an archive to be created at destination. Files
	// are added to the root of the archive, and directories
	// are walked and recursively added, preserving folder
	// structure.
	Archive(sources []string, destination string) error
}

type File = common.File
type FileInfo = common.FileInfo

// ExtensionChecker validates file extensions
type ExtensionChecker interface {
	CheckExt(name string) error
}

// FilenameChecker validates filenames to prevent path traversal attacks
type FilenameChecker interface {
	CheckPath(to, filename string) error
}

// Unarchiver is a type that can extract archive files
// into a folder.
type Unarchiver interface {
	Unarchive(source, destination string) error
}

// Writer can write discrete byte streams of files to
// an output stream.
type Writer interface {
	Create(out io.Writer) error
	Write(f common.File) error
	Close() error
}

// Reader can read discrete byte streams of files from
// an input stream.
type Reader interface {
	Open(in io.Reader, size int64) error
	Read() (common.File, error)
	Close() error
}

// Extractor can extract a specific file from a source
// archive to a specific destination folder on disk.
type Extractor interface {
	Extract(source, target, destination string) error
}

// ReadFakeCloser is an io.Reader that has
// a no-op close method to satisfy the
// io.ReadCloser interface.
type ReadFakeCloser struct {
	io.Reader
}

// Close implements io.Closer.
func (rfc ReadFakeCloser) Close() error { return nil }

// Walker can walk an archive file and return information
// about each item in the archive.
type Walker interface {
	Walk(archive string, walkFn common.WalkFunc) error
}

// ErrStopWalk signals Walk to break without error.
var ErrStopWalk = common.ErrStopWalk

// ErrFormatNotRecognized is an error that will be
// returned if the file is not a valid archive format.
var ErrFormatNotRecognized = fmt.Errorf("format not recognized")

// Compressor compresses to out what it reads from in.
// It also ensures a compatible or matching file extension.
type Compressor interface {
	ExtensionChecker
	Compress(in io.Reader, out io.Writer) error
}

// Decompressor decompresses to out what it reads from in.
type Decompressor interface {
	Decompress(in io.Reader, out io.Writer) error
}

// Matcher is a type that can return whether the given
// file appears to match the implementation's format.
// Implementations should return the file's read position
// to where it was when the method was called.
type Matcher interface {
	Match(io.ReadSeeker) (bool, error)
}

// Archive creates an archive of the source files to a new file at destination.
// The archive format is chosen implicitly by file extension.
func Archive(sources []string, destination string) error {
	aIface, err := ByExtension(destination)
	if err != nil {
		return err
	}
	a, ok := aIface.(Archiver)
	if !ok {
		return fmt.Errorf("format specified by destination filename is not an archive format: %s (%T)", destination, aIface)
	}
	return a.Archive(sources, destination)
}

// Unarchive unarchives the given archive file into the destination folder.
// The archive format is selected implicitly.
func Unarchive(source, destination string) error {
	uaIface, err := ByExtension(source)
	if err != nil {
		return err
	}
	u, ok := uaIface.(Unarchiver)
	if !ok {
		return fmt.Errorf("format specified by source filename is not an archive format: %s (%T)", source, uaIface)
	}
	return u.Unarchive(source, destination)
}

// Walk calls walkFn for each file within the given archive file.
// The archive format is chosen implicitly.
func Walk(archive string, walkFn common.WalkFunc) error {
	wIface, err := ByExtension(archive)
	if err != nil {
		return err
	}
	w, ok := wIface.(Walker)
	if !ok {
		return fmt.Errorf("format specified by archive filename is not a walker format: %s (%T)", archive, wIface)
	}
	return w.Walk(archive, walkFn)
}

// Extract extracts a single file from the given source archive. If the target
// is a directory, the entire folder will be extracted into destination. The
// archive format is chosen implicitly.
func Extract(source, target, destination string) error {
	eIface, err := ByExtension(source)
	if err != nil {
		return err
	}
	e, ok := eIface.(Extractor)
	if !ok {
		return fmt.Errorf("format specified by source filename is not an extractor format: %s (%T)", source, eIface)
	}
	return e.Extract(source, target, destination)
}

// CompressFile is a convenience function to simply compress a file.
// The compression algorithm is selected implicitly based on the
// destination's extension.
func CompressFile(source, destination string) error {
	cIface, err := ByExtension(destination)
	if err != nil {
		return err
	}
	c, ok := cIface.(Compressor)
	if !ok {
		return fmt.Errorf("format specified by destination filename is not a recognized compression algorithm: %s", destination)
	}
	return FileCompressor{Compressor: c}.CompressFile(source, destination)
}

// DecompressFile is a convenience function to simply decompress a file.
// The decompression algorithm is selected implicitly based on the
// source's extension.
func DecompressFile(source, destination string) error {
	cIface, err := ByExtension(source)
	if err != nil {
		return err
	}
	c, ok := cIface.(Decompressor)
	if !ok {
		return fmt.Errorf("format specified by source filename is not a recognized compression algorithm: %s", source)
	}
	return FileCompressor{Decompressor: c}.DecompressFile(source, destination)
}

func writeNewHardLink(fpath string, target string) error {
	err := os.MkdirAll(filepath.Dir(fpath), 0755)
	if err != nil {
		return fmt.Errorf("%s: making directory for file: %v", fpath, err)
	}

	_, err = os.Lstat(fpath)
	if err == nil {
		err = os.Remove(fpath)
		if err != nil {
			return fmt.Errorf("%s: failed to unlink: %+v", fpath, err)
		}
	}

	err = os.Link(target, fpath)
	if err != nil {
		return fmt.Errorf("%s: making hard link for: %v", fpath, err)
	}
	return nil
}

// NameInArchive returns a name for the file at fpath suitable for
// the inside of an archive. The source and its associated sourceInfo
// is the path where walking a directory started, and if no directory
// was walked, source may == fpath. The returned name is essentially
// the components of the path between source and fpath, preserving
// the internal directory structure.
func NameInArchive(sourceInfo os.FileInfo, source, fpath string) (string, error) {
	return common.MakeNameInArchive(sourceInfo, source, "", fpath)
}

// ByExtension returns an archiver and unarchiver, or compressor
// and decompressor, based on the extension of the filename.
func ByExtension(filename string) (interface{}, error) {
	var ec interface{}
	for _, c := range extCheckers {
		if err := c.CheckExt(filename); err == nil {
			ec = c
			break
		}
	}
	switch ec.(type) {
	case *Rar:
		return NewRar(), nil
	case *Tar:
		return NewTar(), nil
	case *TarBrotli:
		return NewTarBrotli(), nil
	case *TarBz2:
		return NewTarBz2(), nil
	case *TarGz:
		return NewTarGz(), nil
	case *TarLz4:
		return NewTarLz4(), nil
	case *TarSz:
		return NewTarSz(), nil
	case *TarXz:
		return NewTarXz(), nil
	case *TarZstd:
		return NewTarZstd(), nil
	case *Zip:
		return NewZip(), nil
	case *Gz:
		return NewGz(), nil
	case *Bz2:
		return NewBz2(), nil
	case *Lz4:
		return NewLz4(), nil
	case *Snappy:
		return NewSnappy(), nil
	case *Xz:
		return NewXz(), nil
	case *Zstd:
		return NewZstd(), nil
	}
	return nil, fmt.Errorf("format unrecognized by filename: %s", filename)
}

// ByHeader returns the unarchiver value that matches the input's
// file header. It does not affect the current read position.
// If the file's header is not a recognized archive format, then
// ErrFormatNotRecognized will be returned.
func ByHeader(input io.ReadSeeker) (Unarchiver, error) {
	var matcher Matcher
	for _, m := range matchers {
		ok, err := m.Match(input)
		if err != nil {
			return nil, fmt.Errorf("matching on format %s: %v", m, err)
		}
		if ok {
			matcher = m
			break
		}
	}
	switch matcher.(type) {
	case *Zip:
		return NewZip(), nil
	case *Tar:
		return NewTar(), nil
	case *Rar:
		return NewRar(), nil
	}
	return nil, ErrFormatNotRecognized
}

// extCheckers is a list of the format implementations
// that can check extensions. Only to be used for
// checking extensions - not any archival operations.
var extCheckers = []ExtensionChecker{
	&TarBrotli{},
	&TarBz2{},
	&TarGz{},
	&TarLz4{},
	&TarSz{},
	&TarXz{},
	&TarZstd{},
	&Rar{},
	&Tar{},
	&Zip{},
	&Brotli{},
	&Gz{},
	&Bz2{},
	&Lz4{},
	&Snappy{},
	&Xz{},
	&Zstd{},
}

var matchers = []Matcher{
	&Rar{},
	&Tar{},
	&Zip{},
}
