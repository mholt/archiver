package archiver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path"
	"strings"

	"github.com/bodgit/sevenzip"
)

func init() {
	RegisterFormat(SevenZip{})

	// looks like the sevenzip package registers a lot of decompressors for us automatically:
	// https://github.com/bodgit/sevenzip/blob/46c5197162c784318b98b9a3f80289a9aa1ca51a/register.go#L38-L61
}

type SevenZip struct {
	// If true, errors encountered during reading or writing
	// a file within an archive will be logged and the
	// operation will continue on remaining files.
	ContinueOnError bool

	// The password, if dealing with an encrypted archive.
	Password string
}

func (z SevenZip) Name() string { return ".7z" }

func (z SevenZip) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), z.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(sevenZipHeader))
	if err != nil {
		return mr, err
	}
	mr.ByStream = bytes.Equal(buf, sevenZipHeader)

	return mr, nil
}

// Archive is not implemented for 7z, but the method exists so that SevenZip satisfies the ArchiveFormat interface.
func (z SevenZip) Archive(_ context.Context, _ io.Writer, _ []File) error {
	return fmt.Errorf("not implemented for 7z because there is no pure Go implementation found")
}

// Extract extracts files from z, implementing the Extractor interface. Uniquely, however,
// sourceArchive must be an io.ReaderAt and io.Seeker, which are oddly disjoint interfaces
// from io.Reader which is what the method signature requires. We chose this signature for
// the interface because we figure you can Read() from anything you can ReadAt() or Seek()
// with. Due to the nature of the zip archive format, if sourceArchive is not an io.Seeker
// and io.ReaderAt, an error is returned.
func (z SevenZip) Extract(ctx context.Context, sourceArchive io.Reader, pathsInArchive []string, handleFile FileHandler) error {
	sra, ok := sourceArchive.(seekReaderAt)
	if !ok {
		return fmt.Errorf("input type must be an io.ReaderAt and io.Seeker because of zip format constraints")
	}

	size, err := streamSizeBySeeking(sra)
	if err != nil {
		return fmt.Errorf("determining stream size: %w", err)
	}

	zr, err := sevenzip.NewReaderWithPassword(sra, size, z.Password)
	if err != nil {
		return err
	}

	// important to initialize to non-nil, empty value due to how fileIsIncluded works
	skipDirs := skipList{}

	for i, f := range zr.File {
		f := f // make a copy for the Open closure
		if err := ctx.Err(); err != nil {
			return err // honor context cancellation
		}

		if !fileIsIncluded(pathsInArchive, f.Name) {
			continue
		}
		if fileIsIncluded(skipDirs, f.Name) {
			continue
		}

		file := File{
			FileInfo:      f.FileInfo(),
			Header:        f.FileHeader,
			NameInArchive: f.Name,
			Open:          func() (io.ReadCloser, error) { return f.Open() },
		}

		err := handleFile(ctx, file)
		if errors.Is(err, fs.SkipDir) {
			// if a directory, skip this path; if a file, skip the folder path
			dirPath := f.Name
			if !file.IsDir() {
				dirPath = path.Dir(f.Name) + "/"
			}
			skipDirs.add(dirPath)
		} else if err != nil {
			if z.ContinueOnError {
				log.Printf("[ERROR] %s: %v", f.Name, err)
				continue
			}
			return fmt.Errorf("handling file %d: %s: %w", i, f.Name, err)
		}
	}

	return nil
}

// https://py7zr.readthedocs.io/en/latest/archive_format.html#signature
var sevenZipHeader = []byte("7z\xBC\xAF\x27\x1C")
