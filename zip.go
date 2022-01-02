package archiver

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/dsnet/compress/bzip2"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

func init() {
	RegisterFormat(Zip{})

	// TODO: What about custom flate levels too
	zip.RegisterCompressor(ZipMethodBzip2, func(out io.Writer) (io.WriteCloser, error) {
		return bzip2.NewWriter(out, &bzip2.WriterConfig{ /*TODO: Level: z.CompressionLevel*/ })
	})
	zip.RegisterCompressor(ZipMethodZstd, func(out io.Writer) (io.WriteCloser, error) {
		return zstd.NewWriter(out)
	})
	zip.RegisterCompressor(ZipMethodXz, func(out io.Writer) (io.WriteCloser, error) {
		return xz.NewWriter(out)
	})

	zip.RegisterDecompressor(ZipMethodBzip2, func(r io.Reader) io.ReadCloser {
		bz2r, err := bzip2.NewReader(r, nil)
		if err != nil {
			return nil
		}
		return bz2r
	})
	zip.RegisterDecompressor(ZipMethodZstd, func(r io.Reader) io.ReadCloser {
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil
		}
		return zr.IOReadCloser()
	})
	zip.RegisterDecompressor(ZipMethodXz, func(r io.Reader) io.ReadCloser {
		xr, err := xz.NewReader(r)
		if err != nil {
			return nil
		}
		return io.NopCloser(xr)
	})
}

type Zip struct {
	// Only compress files which are not already in a
	// compressed format (determined simply by examining
	// file extension).
	SelectiveCompression bool

	// The method or algorithm for compressing stored files.
	Compression uint16

	// If true, errors encountered during reading or writing
	// a file within an archive will be logged and the
	// operation will continue on remaining files.
	ContinueOnError bool
}

func (z Zip) Name() string { return ".zip" }

func (z Zip) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), z.Name()) {
		mr.ByName = true
	}

	// match file header
	buf := make([]byte, len(zipHeader))
	if _, err := io.ReadFull(stream, buf); err != nil {
		return mr, err
	}
	mr.ByStream = bytes.Equal(buf, zipHeader)

	return mr, nil
}

func (z Zip) Archive(ctx context.Context, output io.Writer, files []File) error {
	if ctx == nil {
		ctx = context.Background()
	}

	zw := zip.NewWriter(output)
	defer zw.Close()

	for i, file := range files {
		if err := ctx.Err(); err != nil {
			return err // honor context cancellation
		}

		hdr, err := zip.FileInfoHeader(file)
		if err != nil {
			return fmt.Errorf("getting info for file %d: %s: %w", i, file.Name(), err)
		}

		// customize header based on file properties
		if file.IsDir() {
			hdr.Name += "/" // required - strangely no mention of this in zip spec? but is in godoc...
			hdr.Method = zip.Store
		} else if z.SelectiveCompression {
			// only enable compression on compressable files
			ext := strings.ToLower(path.Ext(hdr.Name))
			if _, ok := compressedFormats[ext]; ok {
				hdr.Method = zip.Store
			} else {
				hdr.Method = z.Compression
			}
		}

		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return fmt.Errorf("creating header for file %d: %s: %w", i, file.Name(), err)
		}

		// directories have no file body
		if file.IsDir() {
			continue
		}
		if err := openAndCopyFile(file, w); err != nil {
			return fmt.Errorf("writing file %d: %s: %w", i, file.Name(), err)
		}
	}

	return nil
}

func (z Zip) Extract(ctx context.Context, sourceArchive io.Reader, pathsInArchive []string, handleFile FileHandler) error {
	if ctx == nil {
		ctx = context.Background()
	}

	sra, ok := sourceArchive.(seekReaderAt)
	if !ok {
		return fmt.Errorf("input type must be an io.ReaderAt and io.Seeker because of zip format constraints")
	}

	size, err := streamSizeBySeeking(sra)
	if err != nil {
		return fmt.Errorf("determining stream size: %w", err)
	}

	zr, err := zip.NewReader(sra, size)
	if err != nil {
		return err
	}

	// important to initialize to non-nil, empty value due to how fileIsIncluded works
	skipDirs := skipList{}

	for i, f := range zr.File {
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
				dirPath = path.Dir(f.Name)
			}
			skipDirs.add(dirPath)
		} else if err != nil {
			return fmt.Errorf("handling file %d: %s: %w", i, f.Name, err)
		}
	}

	return nil
}

type seekReaderAt interface {
	io.ReaderAt
	io.Seeker
}

func streamSizeBySeeking(s io.Seeker) (int64, error) {
	currentPosition, err := s.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("getting current offset: %w", err)
	}
	maxPosition, err := s.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("fast-forwarding to end: %w", err)
	}
	_, err = s.Seek(currentPosition, io.SeekStart)
	if err != nil {
		return 0, fmt.Errorf("returning to prior offset %d: %w", currentPosition, err)
	}
	return maxPosition, nil
}

// Additional compression methods not offered by archive/zip.
// See https://pkware.cachefly.net/webdocs/casestudies/APPNOTE.TXT section 4.4.5.
const (
	ZipMethodBzip2 = 12
	// TODO: LZMA: Disabled - because 7z isn't able to unpack ZIP+LZMA ZIP+LZMA2 archives made this way - and vice versa.
	// ZipMethodLzma     = 14
	ZipMethodZstd = 93
	ZipMethodXz   = 95
)

// compressedFormats is a (non-exhaustive) set of lowercased
// file extensions for formats that are typically already
// compressed. Compressing files that are already compressed
// is inefficient, so use this set of extensions to avoid that.
var compressedFormats = map[string]struct{}{
	".7z":   {},
	".avi":  {},
	".br":   {},
	".bz2":  {},
	".cab":  {},
	".docx": {},
	".gif":  {},
	".gz":   {},
	".jar":  {},
	".jpeg": {},
	".jpg":  {},
	".lz":   {},
	".lz4":  {},
	".lzma": {},
	".m4v":  {},
	".mov":  {},
	".mp3":  {},
	".mp4":  {},
	".mpeg": {},
	".mpg":  {},
	".png":  {},
	".pptx": {},
	".rar":  {},
	".sz":   {},
	".tbz2": {},
	".tgz":  {},
	".tsz":  {},
	".txz":  {},
	".xlsx": {},
	".xz":   {},
	".zip":  {},
	".zipx": {},
}

var zipHeader = []byte("PK\x03\x04") // TODO: headers of empty zip files might end with 0x05,0x06 or 0x06,0x06 instead of 0x03,0x04
