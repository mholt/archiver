package archiver

import "io"

// head returns the first maxBytes from the stream.
// It will return less than maxBytes if the stream does not contain enough data.
// head will happily return an empty array if stream is nil or maxBytes is 0.
func head(stream io.Reader, maxBytes uint) ([]byte, error) {
	if stream == nil || maxBytes == 0 {
		return []byte{}, nil
	}
	buf := make([]byte, maxBytes)
	// we are interested in reading at most maxBytes.
	// This seems to be the same feature as provided by io.Reader.Read().
	// It is not because:
	//   -- io.ReadFull() will put some extra effort to fully read up to the buf size until an EOF.
	//   -- and io.Reader.Read() will not.
	n, err := io.ReadFull(stream, buf)

	// Ignoring the following errors, because they means stream contains less than maxBytes:
	// - io.EOF --> the stream is empty
	// - io.ErrUnexpectedEOF --> the stream has less than mayBytes
	if err != nil && !(err == io.ErrUnexpectedEOF || err == io.EOF) {
		return nil, err
	}
	return buf[:n], nil
}
