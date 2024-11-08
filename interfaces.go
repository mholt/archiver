package archiver

import (
	"context"
	"io"
)

// Format represents a way of getting data out of something else.
// A format usually represents compression or an archive (or both).
type Format interface {
	// Extension returns the conventional file extension for this
	// format.
	Extension() string

	// Match returns true if the given name/stream is recognized.
	// One of the arguments is optional: filename might be empty
	// if working with an unnamed stream, or stream might be
	// empty if only working with a filename. The filename should
	// consist only of the base name, not a path component, and is
	// typically used for matching by file extension. However,
	// matching by reading the stream is preferred. Match reads
	// only as many bytes as needed to determine a match. To
	// preserve the stream through matching, you should either
	// buffer what is read by Match, or seek to the last position
	// before Match was called.
	Match(ctx context.Context, filename string, stream io.Reader) (MatchResult, error)
}

// Compression is a compression format with both compress and decompress methods.
type Compression interface {
	Format
	Compressor
	Decompressor
}

// Archival is an archival format that can create/write archives.
type Archival interface {
	Format
	Archiver
}

// Extraction is an archival format that extract from (read) archives.
type Extraction interface {
	Format
	Extractor
}

// Compressor can compress data by wrapping a writer.
type Compressor interface {
	// OpenWriter wraps w with a new writer that compresses what is written.
	// The writer must be closed when writing is finished.
	OpenWriter(w io.Writer) (io.WriteCloser, error)
}

// Decompressor can decompress data by wrapping a reader.
type Decompressor interface {
	// OpenReader wraps r with a new reader that decompresses what is read.
	// The reader must be closed when reading is finished.
	OpenReader(r io.Reader) (io.ReadCloser, error)
}

// Archiver can create a new archive.
type Archiver interface {
	// Archive writes an archive file to output with the given files.
	//
	// Context cancellation must be honored.
	Archive(ctx context.Context, output io.Writer, files []FileInfo) error
}

// ArchiveAsyncJob contains a File to be archived and a channel that
// the result of the archiving should be returned on.
type ArchiveAsyncJob struct {
	File   FileInfo
	Result chan<- error
}

// ArchiverAsync is an Archiver that can also create archives
// asynchronously by pumping files into a channel as they are
// discovered.
type ArchiverAsync interface {
	Archiver

	// Use ArchiveAsync if you can't pre-assemble a list of all
	// the files for the archive. Close the jobs channel after
	// all the files have been sent.
	//
	// This won't return until the channel is closed.
	ArchiveAsync(ctx context.Context, output io.Writer, jobs <-chan ArchiveAsyncJob) error
}

// Extractor can extract files from an archive.
type Extractor interface {
	// Extract walks entries in the archive and calls handleFile for each
	// entry in the archive.
	//
	// Any files opened in the FileHandler should be closed when it returns,
	// as there is no guarantee the files can be read outside the handler
	// or after the walk has proceeded to the next file.
	//
	// Context cancellation must be honored.
	Extract(ctx context.Context, archive io.Reader, handleFile FileHandler) error
}

// Inserter can insert files into an existing archive.
// EXPERIMENTAL: This API is subject to change.
type Inserter interface {
	// Insert inserts the files into archive.
	//
	// Context cancellation must be honored.
	Insert(ctx context.Context, archive io.ReadWriteSeeker, files []FileInfo) error
}
