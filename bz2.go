package archiver

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/dsnet/compress/bzip2"
	ftmatcher "github.com/h2non/filetype/matchers"
)

// Bz2 facilitates bzip2 compression.
type Bz2 struct {
	CompressionLevel int
}

// Compress reads in, compresses it, and writes it to out.
func (bz *Bz2) Compress(in io.Reader, out io.Writer) error {
	w, err := bzip2.NewWriter(out, &bzip2.WriterConfig{
		Level: bz.CompressionLevel,
	})
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, in)
	return err
}

// Decompress reads in, decompresses it, and writes it to out.
func (bz *Bz2) Decompress(in io.Reader, out io.Writer) error {
	r, err := bzip2.NewReader(in, nil)
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(out, r)
	return err
}

// CheckExt ensures the file extension matches the format.
func (bz *Bz2) CheckExt(filename string) error {
	if filepath.Ext(filename) != ".bz2" {
		return fmt.Errorf("filename must have a .bz2 extension")
	}
	return nil
}

// Match returns true if the format of file matches this
// type's format. It should not affect reader position.
func (bz *Bz2) Match(file io.ReadSeeker) (bool, error) {
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

	return ftmatcher.Bz2(buf), nil
}

func (bz *Bz2) String() string { return "bz2" }

// NewBz2 returns a new, default instance ready to be customized and used.
func NewBz2() *Bz2 {
	return &Bz2{
		CompressionLevel: bzip2.DefaultCompression,
	}
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Compressor(new(Bz2))
	_ = Decompressor(new(Bz2))
)

// DefaultBz2 is a default instance that is conveniently ready to use.
var DefaultBz2 = NewBz2()
