package archiver

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

// Zstd facilitates Zstandard compression.
type Zstd struct {
}

// Compress reads in, compresses it, and writes it to out.
func (zs *Zstd) Compress(in io.Reader, out io.Writer) error {
	w, err := zstd.NewWriter(out)
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, in)
	return err
}

// Decompress reads in, decompresses it, and writes it to out.
func (zs *Zstd) Decompress(in io.Reader, out io.Writer) error {
	r, err := zstd.NewReader(in)
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(out, r)
	return err
}

// CheckExt ensures the file extension matches the format.
func (zs *Zstd) CheckExt(filename string) error {
	if filepath.Ext(filename) != ".zst" {
		return fmt.Errorf("filename must have a .zst extension")
	}
	return nil
}

func (zs *Zstd) String() string { return "zstd" }

func (zs *Zstd) Match(in io.ReadSeeker) (bool, error) {
	currentPos, err := in.Seek(0, io.SeekCurrent)
	if err != nil {
		return false, err
	}

	_, err = in.Seek(0, 0)
	if err != nil {
		return false, err
	}
	defer in.Seek(currentPos, io.SeekStart)

	buf := make([]byte, 4)
	if n, err := in.Read(buf); err != nil || n < 4 {
		return false, nil
	}
	return bytes.Equal(buf, []byte{40, 181, 47, 253}), nil
}

// NewZstd returns a new, default instance ready to be customized and used.
func NewZstd() *Zstd {
	return new(Zstd)
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Compressor(new(Zstd))
	_ = Decompressor(new(Zstd))
)

// DefaultZstd is a default instance that is conveniently ready to use.
var DefaultZstd = NewZstd()
