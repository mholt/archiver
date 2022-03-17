package archiver

import (
	"bytes"
	"io"
	"strings"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/pgzip"
)

func init() {
	RegisterFormat(Gz{})
}

// Gz facilitates gzip compression.
type Gz struct {
	// Gzip compression level. See https://pkg.go.dev/compress/flate#pkg-constants
	// for some predefined constants. If 0, DefaultCompression is assumed rather
	// than no compression.
	CompressionLevel int

	// Use a fast parallel Gzip implementation. This is only
	// effective for large streams (about 1 MB or greater).
	Multithreaded bool
}

func (Gz) Name() string { return ".gz" }

func (gz Gz) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), gz.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(gzHeader))
	if err != nil {
		return mr, err
	}
	mr.ByStream = bytes.Equal(buf, gzHeader)

	return mr, nil
}

func (gz Gz) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	// assume default compression level if 0, rather than no
	// compression, since no compression on a gzipped file
	// doesn't make any sense in our use cases
	level := gz.CompressionLevel
	if level == 0 {
		level = gzip.DefaultCompression
	}

	var wc io.WriteCloser
	var err error
	if gz.Multithreaded {
		wc, err = pgzip.NewWriterLevel(w, level)
	} else {
		wc, err = gzip.NewWriterLevel(w, level)
	}
	return wc, err
}

func (gz Gz) OpenReader(r io.Reader) (io.ReadCloser, error) {
	var rc io.ReadCloser
	var err error
	if gz.Multithreaded {
		rc, err = pgzip.NewReader(r)
	} else {
		rc, err = gzip.NewReader(r)
	}
	return rc, err
}

// magic number at the beginning of gzip files
var gzHeader = []byte{0x1f, 0x8b}
