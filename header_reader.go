package archiver

import (
	"bytes"
	"errors"
	"io"
	"strings"
)

var (
	errReaderFrozen = errors.New("Reader() has been called and reads are now frozen")
)

// headerReader will read from an underlying reader but buffer all the calls
// to Read(). You are then able to reset the reader by calling Rewind() which is equivalent
// to Seek(0,0). This reader does not implement the io.Seeker interface because any other calls
// to Seek would be inefficient and would not be supported by this reader.
//
// Once the header has been read and rewound as much as you would like, call Reader() to
// get a reader that will no longer buffer calls to read. The internal buffer would
// be drained then calls would be redirected back to the underlying reader.
// When calling Reader(), the returned reader will read from the current cursor position.
// Call Rewind() first to reset the cursor to the start of the stream.
type headerReader struct {
	pos int
	buf []byte

	// sticky error
	err error

	r io.Reader
}

func newHeaderReader(r io.Reader) *headerReader {
	const initialBufferSize = 128

	// make sure the underlying reader is non-nil
	if r == nil {
		r = strings.NewReader("")
	}

	return &headerReader{
		buf: make([]byte, 0, initialBufferSize),
		r:   r,
	}
}

func (s *headerReader) Read(data []byte) (n int, err error) {
	if s.err != nil && s.err != io.EOF {
		return 0, s.err
	}

	// if this read is asking for more data than we have buffered
	// then load more data from the underlying reader into the buffer
	if s.pos+len(data) > len(s.buf) {
		s.readUptoNMore(s.pos + len(data) - len(s.buf))
	}

	// copy whats in the buffer into the data slice
	n = copy(data, s.buf[s.pos:])
	s.pos += n

	return n, s.err
}

// Rewind sets the pointer back to the start of the stream.
// Any following calls to Read will come from the start of the stream again
func (s *headerReader) Rewind() { s.pos = 0 }

// Reader returns a reader which will read from the current position in
// the buffer onwards. Use Rewind() first to reset to the start of the
// stream.
//
// Once this function has been called, any subsequent reads to the stream
// header reader will result in ErrReaderFrozen being returned.
func (s *headerReader) Reader() io.Reader {
	s.err = errReaderFrozen
	return io.MultiReader(bytes.NewReader(s.buf[s.pos:]), s.r)
}

// readUptoNMore will read at most n more bytes from the underlying
// reader, storing them into the buffer. The position will not be
// updated but the buffer will be grown.
func (s *headerReader) readUptoNMore(n int) {
	// grow the buffer by the amount of additional data we need
	l := len(s.buf)
	s.buf = append(s.buf, make([]byte, n)...)

	// We could call io.ReadFull here, but instead just let the
	// behaviour of the underlying reader determine how the reads
	// are handled.
	n, s.err = s.r.Read(s.buf[l:])

	// if we read less, make sure the buffer is trimmed
	s.buf = s.buf[:l+n]
}
