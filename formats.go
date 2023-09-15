package archiver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// RegisterFormat registers a format. It should be called during init.
// Duplicate formats by name are not allowed and will panic.
func RegisterFormat(format Format) {
	name := strings.Trim(strings.ToLower(format.Name()), ".")
	if _, ok := formats[name]; ok {
		panic("format " + name + " is already registered")
	}
	formats[name] = format
}

// Identify iterates the registered formats and returns the one that
// matches the given filename and/or stream. It is capable of identifying
// compressed files (.gz, .xz...), archive files (.tar, .zip...), and
// compressed archive files (tar.gz, tar.bz2...). The returned Format
// value can be type-asserted to ascertain its capabilities.
//
// If no matching formats were found, special error ErrNoMatch is returned.
//
// If stream is nil then it will only match on file name and the
// returned io.Reader will be nil.
//
// If stream is non-nil then the returned io.Reader will always be
// non-nil and will read from the same point as the reader which was
// passed in; it should be used in place of the input stream after
// calling Identify() because it preserves and re-reads the bytes that
// were already read during the identification process.
func Identify(filename string, stream io.Reader) (Format, io.Reader, error) {
	var compression Compression
	var archival Archival

	rewindableStream := newRewindReader(stream)

	// try compression format first, since that's the outer "layer"
	for name, format := range formats {
		cf, isCompression := format.(Compression)
		if !isCompression {
			continue
		}

		matchResult, err := identifyOne(format, filename, rewindableStream, nil)
		if err != nil {
			return nil, rewindableStream.reader(), fmt.Errorf("matching %s: %w", name, err)
		}

		// if matched, wrap input stream with decompression
		// so we can see if it contains an archive within
		if matchResult.Matched() {
			compression = cf
			break
		}
	}

	// try archive format next
	for name, format := range formats {
		af, isArchive := format.(Archival)
		if !isArchive {
			continue
		}

		matchResult, err := identifyOne(format, filename, rewindableStream, compression)
		if err != nil {
			return nil, rewindableStream.reader(), fmt.Errorf("matching %s: %w", name, err)
		}

		if matchResult.Matched() {
			archival = af
			break
		}
	}

	// the stream should be rewound by identifyOne
	bufferedStream := rewindableStream.reader()
	switch {
	case compression != nil && archival == nil:
		return compression, bufferedStream, nil
	case compression == nil && archival != nil:
		return archival, bufferedStream, nil
	case compression != nil && archival != nil:
		return CompressedArchive{compression, archival}, bufferedStream, nil
	default:
		return nil, bufferedStream, ErrNoMatch
	}
}

func identifyOne(format Format, filename string, stream *rewindReader, comp Compression) (mr MatchResult, err error) {
	defer stream.rewind()

	// if looking within a compressed format, wrap the stream in a
	// reader that can decompress it so we can match the "inner" format
	// (yes, we have to make a new reader every time we do a match,
	// because we reset/seek the stream each time and that can mess up
	// the compression reader's state if we don't discard it also)
	if comp != nil && stream != nil {
		decompressedStream, openErr := comp.OpenReader(stream)
		if openErr != nil {
			return MatchResult{}, openErr
		}
		defer decompressedStream.Close()
		mr, err = format.Match(filename, decompressedStream)
	} else {
		// Make sure we pass a nil io.Reader not a *rewindReader(nil)
		var r io.Reader
		if stream != nil {
			r = stream
		}
		mr, err = format.Match(filename, r)
	}

	// if the error is EOF, we can just ignore it.
	// Just means we have a small input file.
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return mr, err
}

// readAtMost reads at most n bytes from the stream. A nil, empty, or short
// stream is not an error. The returned slice of bytes may have length < n
// without an error.
func readAtMost(stream io.Reader, n int) ([]byte, error) {
	if stream == nil || n <= 0 {
		return []byte{}, nil
	}

	buf := make([]byte, n)
	nr, err := io.ReadFull(stream, buf)

	// Return the bytes read if there was no error OR if the
	// error was EOF (stream was empty) or UnexpectedEOF (stream
	// had less than n). We ignore those errors because we aren't
	// required to read the full n bytes; so an empty or short
	// stream is not actually an error.
	if err == nil ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) {
		return buf[:nr], nil
	}

	return nil, err
}

// CompressedArchive combines a compression format on top of an archive
// format (e.g. "tar.gz") and provides both functionalities in a single
// type. It ensures that archive functions are wrapped by compressors and
// decompressors. However, compressed archives have some limitations; for
// example, files cannot be inserted/appended because of complexities with
// modifying existing compression state (perhaps this could be overcome,
// but I'm not about to try it).
//
// As this type is intended to compose compression and archive formats,
// both must be specified in order for this value to be valid, or its
// methods will return errors.
type CompressedArchive struct {
	Compression
	Archival
}

// Name returns a concatenation of the archive format name
// and the compression format name.
func (caf CompressedArchive) Name() string {
	if caf.Compression == nil && caf.Archival == nil {
		panic("missing both compression and archive formats")
	}
	var name string
	if caf.Archival != nil {
		name += caf.Archival.Name()
	}
	if caf.Compression != nil {
		name += caf.Compression.Name()
	}
	return name
}

// Match matches if the input matches both the compression and archive format.
func (caf CompressedArchive) Match(filename string, stream io.Reader) (MatchResult, error) {
	var conglomerate MatchResult

	if caf.Compression != nil {
		matchResult, err := caf.Compression.Match(filename, stream)
		if err != nil {
			return MatchResult{}, err
		}
		if !matchResult.Matched() {
			return matchResult, nil
		}

		// wrap the reader with the decompressor so we can
		// attempt to match the archive by reading the stream
		rc, err := caf.Compression.OpenReader(stream)
		if err != nil {
			return matchResult, err
		}
		defer rc.Close()
		stream = rc

		conglomerate = matchResult
	}

	if caf.Archival != nil {
		matchResult, err := caf.Archival.Match(filename, stream)
		if err != nil {
			return MatchResult{}, err
		}
		if !matchResult.Matched() {
			return matchResult, nil
		}
		conglomerate.ByName = conglomerate.ByName || matchResult.ByName
		conglomerate.ByStream = conglomerate.ByStream || matchResult.ByStream
	}

	return conglomerate, nil
}

// Archive adds files to the output archive while compressing the result.
func (caf CompressedArchive) Archive(ctx context.Context, output io.Writer, files []File) error {
	if caf.Compression != nil {
		wc, err := caf.Compression.OpenWriter(output)
		if err != nil {
			return err
		}
		defer wc.Close()
		output = wc
	}
	return caf.Archival.Archive(ctx, output, files)
}

// ArchiveAsync adds files to the output archive while compressing the result asynchronously.
func (caf CompressedArchive) ArchiveAsync(ctx context.Context, output io.Writer, jobs <-chan ArchiveAsyncJob) error {
	do, ok := caf.Archival.(ArchiverAsync)
	if !ok {
		return fmt.Errorf("%s archive does not support async writing", caf.Name())
	}
	if caf.Compression != nil {
		wc, err := caf.Compression.OpenWriter(output)
		if err != nil {
			return err
		}
		defer wc.Close()
		output = wc
	}
	return do.ArchiveAsync(ctx, output, jobs)
}

// Extract reads files out of an archive while decompressing the results.
// If Extract is not called from ArchiveFS.Open, then the FileHandler passed
// in must close all opened files by the time the Extract walk finishes.
func (caf CompressedArchive) Extract(ctx context.Context, sourceArchive io.Reader, pathsInArchive []string, handleFile FileHandler) error {
	if caf.Compression != nil {
		rc, err := caf.Compression.OpenReader(sourceArchive)
		if err != nil {
			return err
		}
		// I don't like this solution, but we have to close the decompressor.
		// The problem is that if we simply defer rc.Close(), we potentially
		// close it before the caller is done using files it opened. Ideally
		// it should be closed when the sourceArchive is also closed. But since
		// we don't originate sourceArchive, we can't close it when it closes.
		// The best I can think of for now is this hack where we tell a type
		// that supports this to close another reader when itself closes.
		// See issue #365.
		if cc, ok := sourceArchive.(compressorCloser); ok {
			cc.closeCompressor(rc)
		} else {
			defer rc.Close()
		}
		sourceArchive = rc
	}
	return caf.Archival.Extract(ctx, sourceArchive, pathsInArchive, handleFile)
}

// MatchResult returns true if the format was matched either
// by name, stream, or both. Name usually refers to matching
// by file extension, and stream usually refers to reading
// the first few bytes of the stream (its header). A stream
// match is generally stronger, as filenames are not always
// indicative of their contents if they even exist at all.
type MatchResult struct {
	ByName, ByStream bool
}

// Matched returns true if a match was made by either name or stream.
func (mr MatchResult) Matched() bool { return mr.ByName || mr.ByStream }

// rewindReader is a Reader that can be rewound (reset) to re-read what
// was already read and then continue to read more from the underlying
// stream. When no more rewinding is necessary, call reader() to get a
// new reader that first reads the buffered bytes, then continues to
// read from the stream. This is useful for "peeking" a stream an
// arbitrary number of bytes. Loosely based on the Connection type
// from https://github.com/mholt/caddy-l4.
type rewindReader struct {
	io.Reader
	buf       *bytes.Buffer
	bufReader io.Reader
}

func newRewindReader(r io.Reader) *rewindReader {
	if r == nil {
		return nil
	}
	return &rewindReader{
		Reader: r,
		buf:    new(bytes.Buffer),
	}
}

func (rr *rewindReader) Read(p []byte) (n int, err error) {
	if rr == nil {
		panic("internal error: reading from nil rewindReader")
	}
	// if there is a buffer we should read from, start
	// with that; we only read from the underlying stream
	// after the buffer has been "depleted"
	if rr.bufReader != nil {
		n, err = rr.bufReader.Read(p)
		if err == io.EOF {
			rr.bufReader = nil
			err = nil
		}
		if n == len(p) {
			return
		}
	}

	// buffer has been "depleted" so read from
	// underlying connection
	nr, err := rr.Reader.Read(p[n:])

	// anything that was read needs to be written to
	// the buffer, even if there was an error
	if nr > 0 {
		if nw, errw := rr.buf.Write(p[n : n+nr]); errw != nil {
			return nw, errw
		}
	}

	// up to now, n was how many bytes were read from
	// the buffer, and nr was how many bytes were read
	// from the stream; add them to return total count
	n += nr

	return
}

// rewind resets the stream to the beginning by causing
// Read() to start reading from the beginning of the
// buffered bytes.
func (rr *rewindReader) rewind() {
	if rr == nil {
		return
	}
	rr.bufReader = bytes.NewReader(rr.buf.Bytes())
}

// reader returns a reader that reads first from the buffered
// bytes, then from the underlying stream. After calling this,
// no more rewinding is allowed since reads from the stream are
// not recorded, so rewinding properly is impossible.
// If the underlying reader implements io.Seeker, then the
// underlying reader will be used directly.
func (rr *rewindReader) reader() io.Reader {
	if rr == nil {
		return nil
	}
	if ras, ok := rr.Reader.(io.Seeker); ok {
		if _, err := ras.Seek(0, io.SeekStart); err == nil {
			return rr.Reader
		}
	}
	return io.MultiReader(bytes.NewReader(rr.buf.Bytes()), rr.Reader)
}

// ErrNoMatch is returned if there are no matching formats.
var ErrNoMatch = fmt.Errorf("no formats matched")

// Registered formats.
var formats = make(map[string]Format)

// Interface guards
var (
	_ Format        = (*CompressedArchive)(nil)
	_ Archiver      = (*CompressedArchive)(nil)
	_ ArchiverAsync = (*CompressedArchive)(nil)
	_ Extractor     = (*CompressedArchive)(nil)
)
