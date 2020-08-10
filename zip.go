package archiver

import (
	"github.com/mholt/archiver/v3/zip"
)

// ZipCompressionMethod Compression type
type ZipCompressionMethod = zip.ZipCompressionMethod

// Compression methods.
// see https://pkware.cachefly.net/webdocs/casestudies/APPNOTE.TXT.
// Note LZMA: Disabled - because 7z isn't able to unpack ZIP+LZMA ZIP+LZMA2 archives made this way - and vice versa.
const (
	Store   ZipCompressionMethod = zip.Store
	Deflate ZipCompressionMethod = zip.Deflate
	BZIP2   ZipCompressionMethod = zip.BZIP2
	LZMA    ZipCompressionMethod = zip.LZMA
	ZSTD    ZipCompressionMethod = zip.ZSTD
	XZ      ZipCompressionMethod = zip.XZ
)

type Zip = zip.Zip

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Reader(new(Zip))
	_ = Writer(new(Zip))
	_ = Archiver(new(Zip))
	_ = Unarchiver(new(Zip))
	_ = Walker(new(Zip))
	_ = Extractor(new(Zip))
	_ = Matcher(new(Zip))
	_ = ExtensionChecker(new(Zip))
	_ = FilenameChecker(new(Zip))
)

// NewZip returns a new, default instance ready to be customized and used.
var NewZip = zip.New

// DefaultZip is a default instance that is conveniently ready to use.
var DefaultZip = zip.New()
