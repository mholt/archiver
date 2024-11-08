package archiver

import (
	"context"
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

func (Brotli) Extension() string { return ".br" }

func (br Brotli) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), br.Extension()) {
		mr.ByName = true
	}

	// brotli does not have well-defined file headers or a magic number;
	// the best way to match the stream is probably to try decoding part
	// of it, but we'll just have to guess a large-enough size that is
	// still small enough for the smallest streams we'll encounter
	r := brotli.NewReader(stream)
	buf := make([]byte, 16)
	if _, err := io.ReadFull(r, buf); err == nil {
		mr.ByStream = true
	}

	return mr, nil
}

func (br Brotli) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	return brotli.NewWriterLevel(w, br.Quality), nil
}

func (Brotli) OpenReader(r io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(brotli.NewReader(r)), nil
}
