package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/flate"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver"
	"github.com/nwaples/rardecode"
)

var (
	compressionLevel       int
	overwriteExisting      bool
	mkdirAll               bool
	selectiveCompression   bool
	implicitTopLevelFolder bool
	continueOnError        bool
)

func init() {
	flag.IntVar(&compressionLevel, "level", flate.DefaultCompression, "Compression level")
	flag.BoolVar(&overwriteExisting, "overwrite", false, "Overwrite existing files")
	flag.BoolVar(&mkdirAll, "mkdirs", false, "Make all necessary directories")
	flag.BoolVar(&selectiveCompression, "smart", true, "Only compress files which are not already compressed (zip only)")
	flag.BoolVar(&implicitTopLevelFolder, "folder-safe", true, "If an archive does not have a single top-level folder, create one implicitly")
	flag.BoolVar(&continueOnError, "allow-errors", true, "Log errors and continue processing")
}

func main() {
	if len(os.Args) >= 2 &&
		(os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help") {
		fmt.Println(usageString())
		os.Exit(0)
	}
	if len(os.Args) < 3 {
		fatal(usageString())
	}
	flag.Parse()

	subcommand := flag.Arg(0)

	// get the format we're working with
	iface, err := getFormat(subcommand)
	if err != nil {
		fatal(err)
	}

	// run the desired command
	switch subcommand {
	case "archive":
		a, ok := iface.(archiver.Archiver)
		if !ok {
			fatalf("the archive command does not support the %s format", iface)
		}
		err = a.Archive(flag.Args()[2:], flag.Arg(1))

	case "unarchive":
		a, ok := iface.(archiver.Unarchiver)
		if !ok {
			fatalf("the unarchive command does not support the %s format", iface)
		}
		err = a.Unarchive(flag.Arg(1), flag.Arg(2))

	case "extract":
		e, ok := iface.(archiver.Extractor)
		if !ok {
			fatalf("the extract command does not support the %s format", iface)
		}
		err = e.Extract(flag.Arg(1), flag.Arg(2), flag.Arg(3))

	case "ls":
		w, ok := iface.(archiver.Walker)
		if !ok {
			fatalf("the ls command does not support the %s format", iface)
		}

		var count int
		err = w.Walk(flag.Arg(1), func(f archiver.File) error {
			count++
			switch h := f.Header.(type) {
			case zip.FileHeader:
				fmt.Printf("%s\t%d\t%d\t%s\t%s\n",
					f.Mode(),
					h.Method,
					f.Size(),
					f.ModTime(),
					h.Name,
				)
			case *tar.Header:
				fmt.Printf("%s\t%s\t%s\t%d\t%s\t%s\n",
					f.Mode(),
					h.Uname,
					h.Gname,
					f.Size(),
					f.ModTime(),
					h.Name,
				)

			case *rardecode.FileHeader:
				fmt.Printf("%s\t%d\t%d\t%s\t%s\n",
					f.Mode(),
					int(h.HostOS),
					f.Size(),
					f.ModTime(),
					h.Name,
				)

			default:
				fmt.Printf("%s\t%d\t%s\t?/%s\n",
					f.Mode(),
					f.Size(),
					f.ModTime(),
					f.Name(), // we don't know full path from this
				)
			}
			return nil
		})

		fmt.Printf("total %d", count)

	case "compress":
		c, ok := iface.(archiver.Compressor)
		if !ok {
			fatalf("the compress command does not support the %s format", iface)
		}
		fc := archiver.FileCompressor{Compressor: c}

		in := flag.Arg(1)
		out := flag.Arg(2)

		var deleteWhenDone bool
		if cs, ok := c.(fmt.Stringer); ok && out == cs.String() {
			out = in + "." + out
			deleteWhenDone = true
		}

		err = fc.CompressFile(in, out)
		if err == nil && deleteWhenDone {
			err = os.Remove(in)
		}

	case "decompress":
		c, ok := iface.(archiver.Decompressor)
		if !ok {
			fatalf("the compress command does not support the %s format", iface)
		}
		fc := archiver.FileCompressor{Decompressor: c}

		in := flag.Arg(1)
		out := flag.Arg(2)

		var deleteWhenDone bool
		if cs, ok := c.(fmt.Stringer); ok && out == "" {
			out = strings.TrimSuffix(in, "."+cs.String())
			deleteWhenDone = true
		}

		err = fc.DecompressFile(in, out)
		if err == nil && deleteWhenDone {
			err = os.Remove(in)
		}

	default:
		fatalf("unrecognized command: %s", flag.Arg(0))
	}
	if err != nil {
		fatal(err)
	}
}

func getFormat(subcommand string) (interface{}, error) {
	formatPos := 1
	if subcommand == "compress" {
		formatPos = 2
	}

	// figure out which file format we're working with
	var ext string
	archiveName := flag.Arg(formatPos)
	for _, format := range supportedFormats {
		// match by extension, or, in the case of 'compress',
		// check the format without the leading dot; it allows
		// a shortcut to specify a format while replacing
		// the original file on disk
		if strings.HasSuffix(archiveName, format) ||
			(subcommand == "compress" &&
				archiveName == strings.TrimPrefix(format, ".")) {
			ext = format
			break
		}
	}

	// configure an archiver
	var iface interface{}
	mytar := &archiver.Tar{
		OverwriteExisting:      overwriteExisting,
		MkdirAll:               mkdirAll,
		ImplicitTopLevelFolder: implicitTopLevelFolder,
		ContinueOnError:        continueOnError,
	}

	switch ext {
	case ".rar":
		iface = &archiver.Rar{
			OverwriteExisting:      overwriteExisting,
			MkdirAll:               mkdirAll,
			ImplicitTopLevelFolder: implicitTopLevelFolder,
			ContinueOnError:        continueOnError,
			Password:               os.Getenv("ARCHIVE_PASSWORD"),
		}

	case ".tar":
		iface = mytar

	case ".tbz2":
		fallthrough
	case ".tar.bz2":
		iface = &archiver.TarBz2{
			Tar: mytar,
		}

	case ".tgz":
		fallthrough
	case ".tar.gz":
		iface = &archiver.TarGz{
			Tar:              mytar,
			CompressionLevel: compressionLevel,
		}

	case ".tlz4":
		fallthrough
	case ".tar.lz4":
		iface = &archiver.TarLz4{
			Tar:              mytar,
			CompressionLevel: compressionLevel,
		}

	case ".tsz":
		fallthrough
	case ".tar.sz":
		iface = &archiver.TarSz{
			Tar: mytar,
		}

	case ".txz":
		fallthrough
	case ".tar.xz":
		iface = &archiver.TarXz{
			Tar: mytar,
		}

	case ".zip":
		iface = &archiver.Zip{
			CompressionLevel:       compressionLevel,
			OverwriteExisting:      overwriteExisting,
			MkdirAll:               mkdirAll,
			SelectiveCompression:   selectiveCompression,
			ImplicitTopLevelFolder: implicitTopLevelFolder,
			ContinueOnError:        continueOnError,
		}

	case ".gz":
		iface = &archiver.Gz{
			CompressionLevel: compressionLevel,
		}

	case ".bz2":
		iface = &archiver.Bz2{
			CompressionLevel: compressionLevel,
		}

	case ".lz4":
		iface = &archiver.Lz4{
			CompressionLevel: compressionLevel,
		}

	case ".sz":
		iface = &archiver.Snappy{}

	case ".xz":
		iface = &archiver.Xz{}

	default:
		archiveExt := filepath.Ext(archiveName)
		if archiveExt == "" {
			return nil, fmt.Errorf("format missing (use file extension to specify archive/compression format)")
		}
		return nil, fmt.Errorf("unsupported format '%s'", archiveExt)
	}

	return iface, nil
}

func fatal(v ...interface{}) {
	fmt.Fprintln(os.Stderr, v...)
	os.Exit(1)
}

func fatalf(s string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, s+"\n", v...)
	os.Exit(1)
}

func usageString() string {
	buf := new(bytes.Buffer)
	buf.WriteString(usage)
	flag.CommandLine.SetOutput(buf)
	flag.CommandLine.PrintDefaults()
	return buf.String()
}

// supportedFormats is the list of recognized
// file extensions. They are in an ordered slice
// because ordering is important, since some
// extensions can be substrings of others.
var supportedFormats = []string{
	".tar.bz2",
	".tar.gz",
	".tar.lz4",
	".tar.sz",
	".tar.xz",
	".rar",
	".tar",
	".zip",
	".gz",
	".bz2",
	".lz4",
	".sz",
	".xz",
}

const usage = `Usage: arc {archive|unarchive|extract|ls|compress|decompress|help} [arguments...]
  archive
    Create a new archive file. List the files/folders
    to include in the archive; at least one required.
  unarchive
    Extract an archive file. Provide the archive to
    open and the destination folder to extract into.
  extract
    Extract a single file or folder (recursively) from
    an archive. First argument is the source archive,
    second is the file to extract (exact path within the
    archive is required), and third is destination.
  ls
    List the contents of the archive.
  compress
    Compresses a file, destination optional.
  decompress
    Decompresses a file, destination optional.
  help
    Display this help text. Also -h or --help.

  SPECIFYING THE ARCHIVE FORMAT
    The format of the archive is determined by its
    file extension. Supported extensions:
      .zip
      .tar
      .tar.gz
      .tgz
      .tar.bz2
      .tbz2
      .tar.xz
      .txz
      .tar.lz4
      .tlz4
      .tar.sz
      .tsz
      .rar (open only)
      .bz2
      .gz
      .lz4
      .sz
      .xz

  (DE)COMPRESSING SINGLE FILES
    Some formats are compression-only, and can be used
    with the compress and decompress commands on a
    single file; they do not bundle multiple files.

    To replace a file when compressing, specify the
    source file name for the first argument, and the
    compression format (without leading dot) for the
    second argument. To replace a file when decompressing,
    specify only the source file and no destination.

  PASSWORD-PROTECTED RAR FILES
    Export the ARCHIVE_PASSWORD environment variable
    to be able to open password-protected rar archives.

  GLOBAL FLAG REFERENCE
    The following global flags may be used before the
    sub-command (some flags are format-specific):

`
