package archiver

import (
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"

	ftmatcher "github.com/h2non/filetype/matchers"
	pgzip "github.com/klauspost/pgzip"
)

// Gz facilitates gzip compression.
type Gz struct {
	CompressionLevel int
	SingleThreaded   bool
}

// Compress reads in, compresses it, and writes it to out.
func (gz *Gz) Compress(in io.Reader, out io.Writer) error {
	var w io.WriteCloser
	var err error
	if gz.SingleThreaded {
		w, err = gzip.NewWriterLevel(out, gz.CompressionLevel)
	} else {
		w, err = pgzip.NewWriterLevel(out, gz.CompressionLevel)
	}
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, in)
	return err
}

// Decompress reads in, decompresses it, and writes it to out.
func (gz *Gz) Decompress(in io.Reader, out io.Writer) error {
	var r io.ReadCloser
	var err error
	if gz.SingleThreaded {
		r, err = gzip.NewReader(in)
	} else {
		r, err = pgzip.NewReader(in)
	}
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(out, r)
	return err
}

// CheckExt ensures the file extension matches the format.
func (gz *Gz) CheckExt(filename string) error {
	if filepath.Ext(filename) != ".gz" {
		return fmt.Errorf("filename must have a .gz extension")
	}
	return nil
}

// Match returns true if the format of file matches this
// type's format. It should not affect reader position.
func (gz *Gz) Match(file io.ReadSeeker) (bool, error) {
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return false, err
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return false, err
	}
	defer file.Seek(currentPos, io.SeekStart)

	var buf = make([]byte, 3)
	if _, err = io.ReadFull(file, buf); err != nil {
		return false, nil
	}

	return ftmatcher.Gz(buf), nil
}

func (gz *Gz) String() string { return "gz" }

// NewGz returns a new, default instance ready to be customized and used.
func NewGz() *Gz {
	return &Gz{
		CompressionLevel: gzip.DefaultCompression,
	}
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Compressor(new(Gz))
	_ = Decompressor(new(Gz))
)

// DefaultGz is a default instance that is conveniently ready to use.
var DefaultGz = NewGz()
