package archiver

import (
	"bytes"
	"io"
	"strings"

	"github.com/dsnet/compress/bzip2"
)

func init() {
	RegisterFormat(Bz2{})
}

// Bz2 facilitates bzip2 compression.
type Bz2 struct {
	CompressionLevel int
}

func (Bz2) Name() string { return ".bz2" }

func (bz Bz2) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), bz.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(bzip2Header))
	if err != nil {
		return mr, err
	}
	mr.ByStream = bytes.Equal(buf, bzip2Header)

	return mr, nil
}

func (bz Bz2) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	return bzip2.NewWriter(w, &bzip2.WriterConfig{
		Level: bz.CompressionLevel,
	})
}

func (Bz2) OpenReader(r io.Reader) (io.ReadCloser, error) {
	return bzip2.NewReader(r, nil)
}

var bzip2Header = []byte("BZh")
