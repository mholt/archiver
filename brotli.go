package archiver

import (
	"io"
	"strings"

	"github.com/andybalholm/brotli"
)

func init() {
	RegisterFormat(Brotli{})
}

// Brotli facilitates brotli compression.
type Brotli struct {
	Quality int
}

func (Brotli) Name() string { return ".br" }

func (br Brotli) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), br.Name()) {
		mr.ByName = true
	}

	// brotli does not have well-defined file headers; the
	// best way to match the stream would be to try decoding
	// part of it, and this is not implemented for now

	return mr, nil
}

func (br Brotli) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	return brotli.NewWriterLevel(w, br.Quality), nil
}

func (Brotli) OpenReader(r io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(brotli.NewReader(r)), nil
}
