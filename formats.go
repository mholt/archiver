package archiver

import (
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
func Identify(filename string, stream io.ReadSeeker) (Format, error) {
	var compression Compression
	var archival Archival

	// try compression format first, since that's the outer "layer"
	for name, format := range formats {
		cf, isCompression := format.(Compression)
		if !isCompression {
			continue
		}

		matchResult, err := identifyOne(format, filename, stream, nil)
		if err != nil {
			return nil, fmt.Errorf("matching %s: %w", name, err)
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

		matchResult, err := identifyOne(format, filename, stream, compression)
		if err != nil {
			return nil, fmt.Errorf("matching %s: %w", name, err)
		}

		if matchResult.Matched() {
			archival = af
			break
		}
	}

	switch {
	case compression != nil && archival == nil:
		return compression, nil
	case compression == nil && archival != nil:
		return archival, nil
	case compression != nil && archival != nil:
		return CompressedArchive{compression, archival}, nil
	default:
		return nil, ErrNoMatch
	}
}

func identifyOne(format Format, filename string, stream io.ReadSeeker, comp Compression) (MatchResult, error) {
	if stream == nil {
		// shimming an empty stream is easier than hoping every format's
		// implementation of Match() expects and handles a nil stream
		stream = strings.NewReader("")
	}

	// reset stream position to beginning, then restore current position when done
	previousOffset, err := stream.Seek(0, io.SeekCurrent)
	if err != nil {
		return MatchResult{}, err
	}
	_, err = stream.Seek(0, io.SeekStart)
	if err != nil {
		return MatchResult{}, err
	}
	defer stream.Seek(previousOffset, io.SeekStart)

	// if looking within a compressed format, wrap the stream in a
	// reader that can decompress it so we can match the "inner" format
	// (yes, we have to make a new reader every time we do a match,
	// because we reset/seek the stream each time and that can mess up
	// the compression reader's state if we don't discard it also)
	if comp != nil {
		decompressedStream, err := comp.OpenReader(stream)
		if err != nil {
			return MatchResult{}, err
		}
		defer decompressedStream.Close()
		stream = struct {
			io.Reader
			io.Seeker
		}{
			Reader: decompressedStream,
			Seeker: stream,
		}
	}

	return format.Match(filename, stream)
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

// Extract reads files out of an archive while decompressing the results.
func (caf CompressedArchive) Extract(ctx context.Context, sourceArchive io.Reader, pathsInArchive []string, handleFile FileHandler) error {
	if caf.Compression != nil {
		rc, err := caf.Compression.OpenReader(sourceArchive)
		if err != nil {
			return err
		}
		defer rc.Close()
		sourceArchive = rc
	}
	return caf.Archival.(Extractor).Extract(ctx, sourceArchive, pathsInArchive, handleFile)
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

// ErrNoMatch is returned if there are no matching formats.
var ErrNoMatch = fmt.Errorf("no formats matched")

// Registered formats.
var formats = make(map[string]Format)

// Interface guards
var (
	_ Format    = (*CompressedArchive)(nil)
	_ Archiver  = (*CompressedArchive)(nil)
	_ Extractor = (*CompressedArchive)(nil)
)
