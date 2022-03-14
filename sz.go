package archiver

import (
	"bytes"
	"io"
	"strings"

	"github.com/golang/snappy"
)

func init() {
	RegisterFormat(Sz{})
}

// Sz facilitates Snappy compression.
type Sz struct{}

func (sz Sz) Name() string { return ".sz" }

func (sz Sz) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), sz.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(snappyHeader))
	if err != nil {
		return mr, err
	}
	mr.ByStream = bytes.Equal(buf, snappyHeader)

	return mr, nil
}

func (Sz) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	return snappy.NewBufferedWriter(w), nil
}

func (Sz) OpenReader(r io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(snappy.NewReader(r)), nil
}

// https://github.com/google/snappy/blob/master/framing_format.txt
var snappyHeader = []byte{0xff, 0x06, 0x00, 0x00, 0x73, 0x4e, 0x61, 0x50, 0x70, 0x59}
