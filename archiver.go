package archiver

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// Archiver is a type that can create an archive file
// from a list of source file names.
type Archiver interface {
	// Archive adds all the files or folders in sources
	// to an archive to be created at destination. Files
	// are added to the root of the archive, and directories
	// are walked and recursively added, preserving folder
	// structure.
	Archive(sources []string, destination string) error
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
	Write(f File) error
	Close() error
}

// Reader can read discrete byte streams of files from
// an input stream.
type Reader interface {
	Open(in io.Reader, size int64) error
	Read() (File, error)
	Close() error
}

// Extractor can extract a specific file from a source
// archive to a specific destination folder on disk.
type Extractor interface {
	Extract(source, target, destination string) error
}

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
	Walk(archive string, walkFn WalkFunc) error
}

// WalkFunc is called at each item visited by Walk.
// If an error is returned, the walk may continue
// if the Walker is configured to continue on error.
// The sole exception is the error value ErrStopWalk,
// which stops the walk without an actual error.
type WalkFunc func(f File) error

// ErrStopWalk signals Walk to break without error.
var ErrStopWalk = fmt.Errorf("walk stopped")

// Compressor compresses to out what it reads from in.
// It also ensures a compatible or matching file extension.
type Compressor interface {
	Compress(in io.Reader, out io.Writer) error
	CheckExt(filename string) error
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
	Match(*os.File) (bool, error)
}

func fileExists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

func mkdir(dirPath string) error {
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return fmt.Errorf("%s: making directory: %v", dirPath, err)
	}
	return nil
}

func writeNewFile(fpath string, in io.Reader, fm os.FileMode) error {
	err := os.MkdirAll(filepath.Dir(fpath), 0755)
	if err != nil {
		return fmt.Errorf("%s: making directory for file: %v", fpath, err)
	}

	out, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("%s: creating new file: %v", fpath, err)
	}
	defer out.Close()

	err = out.Chmod(fm)
	if err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("%s: changing file mode: %v", fpath, err)
	}

	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("%s: writing file: %v", fpath, err)
	}
	return nil
}

func writeNewSymbolicLink(fpath string, target string) error {
	err := os.MkdirAll(filepath.Dir(fpath), 0755)
	if err != nil {
		return fmt.Errorf("%s: making directory for file: %v", fpath, err)
	}

	err = os.Symlink(target, fpath)
	if err != nil {
		return fmt.Errorf("%s: making symbolic link for: %v", fpath, err)
	}

	return nil
}

func writeNewHardLink(fpath string, target string) error {
	err := os.MkdirAll(filepath.Dir(fpath), 0755)
	if err != nil {
		return fmt.Errorf("%s: making directory for file: %v", fpath, err)
	}

	err = os.Link(target, fpath)
	if err != nil {
		return fmt.Errorf("%s: making hard link for: %v", fpath, err)
	}

	return nil
}

// within returns true if sub is within or equal to parent.
func within(parent, sub string) bool {
	rel, err := filepath.Rel(parent, sub)
	if err != nil {
		return false
	}
	return !strings.Contains(rel, "..")
}

// multipleTopLevels returns true if the paths do not
// share a common top-level folder.
func multipleTopLevels(paths []string) bool {
	if len(paths) < 2 {
		return false
	}
	var lastTop string
	for _, p := range paths {
		p = strings.TrimPrefix(strings.Replace(p, `\`, "/", -1), "/")
		for {
			next := path.Dir(p)
			if next == "." {
				break
			}
			p = next
		}
		if lastTop == "" {
			lastTop = p
		}
		if p != lastTop {
			return true
		}
	}
	return false
}

// folderNameFromFileName returns a name for a folder
// that is suitable based on the filename, which will
// be stripped of its extensions.
func folderNameFromFileName(filename string) string {
	base := filepath.Base(filename)
	firstDot := strings.Index(base, ".")
	if firstDot > -1 {
		return base[:firstDot]
	}
	return base
}

// makeNameInArchive returns the filename for the file given by fpath to be used within
// the archive. sourceInfo is the FileInfo obtained by calling os.Stat on source, and baseDir
// is an optional base directory that becomes the root of the archive. fpath should be the
// unaltered file path of the file given to a filepath.WalkFunc.
func makeNameInArchive(sourceInfo os.FileInfo, source, baseDir, fpath string) (string, error) {
	name := filepath.Base(fpath) // start with the file or dir name
	if sourceInfo.IsDir() {
		// preserve internal directory structure; that's the path components
		// between the source directory's leaf and this file's leaf
		dir, err := filepath.Rel(filepath.Dir(source), filepath.Dir(fpath))
		if err != nil {
			return "", err
		}
		// prepend the internal directory structure to the leaf name,
		// and convert path separators to forward slashes as per spec
		name = path.Join(filepath.ToSlash(dir), name)
	}
	return path.Join(baseDir, name), nil // prepend the base directory
}
