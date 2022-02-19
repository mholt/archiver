package archiver

import (
	"bytes"
	"io"
	"testing"
)

func TestStreamHeaderReader(t *testing.T) {
	data := []byte("the header\nthe body\n")

	r := newHeaderReader(bytes.NewReader(data))

	buf := make([]byte, 10) // enough for 'the header'

	// test rewinding reads
	for i := 0; i < 10; i++ {
		r.Rewind()
		_, err := r.Read(buf)
		if err != nil {
			t.Fatalf("Read failed: %s", err)
		}
		if string(buf) != "the header" {
			t.Fatalf("expected 'the header' but got '%s'", string(buf))
		}
	}

	// get the reader from header reader and make sure we can read all of the data out
	r.Rewind()
	finalReader := r.Reader()
	buf = make([]byte, len(data))
	_, err := io.ReadFull(finalReader, buf)
	if err != nil {
		t.Fatalf("ReadFull failed: %s", err)
	}

	if string(buf) != string(data) {
		t.Fatalf("expected '%s' but got '%s'", string(data), string(buf))
	}
}
