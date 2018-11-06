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

	"github.com/nwaples/rardecode"

	"github.com/mholt/archiver/archive"
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
	if len(os.Args) == 2 &&
		(os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help") {
		buf := new(bytes.Buffer)
		flag.CommandLine.SetOutput(buf)
		buf.WriteString(usage)
		flag.CommandLine.PrintDefaults()
		fmt.Println(buf.String())
		os.Exit(0)
	}
	if len(os.Args) < 3 {
		fatal(usage)
	}
	flag.Parse()

	// figure out which file format we're working with
	var ext string
	archiveName := flag.Arg(1)
	for _, format := range supportedFormats {
		if strings.HasSuffix(archiveName, format) {
			ext = format
			break
		}
	}

	// configure an archiver
	var iface interface{}
	mytar := &archive.Tar{
		OverwriteExisting:      overwriteExisting,
		MkdirAll:               mkdirAll,
		ImplicitTopLevelFolder: implicitTopLevelFolder,
		ContinueOnError:        continueOnError,
	}
	switch ext {
	case ".rar":
		iface = &archive.Rar{
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
		iface = &archive.TarBz2{
			Tar: mytar,
		}

	case ".tgz":
		fallthrough
	case ".tar.gz":
		iface = &archive.TarGz{
			Tar:              mytar,
			CompressionLevel: compressionLevel,
		}

	case ".tlz4":
		fallthrough
	case ".tar.lz4":
		iface = &archive.TarLz4{
			Tar:              mytar,
			CompressionLevel: compressionLevel,
		}

	case ".tsz":
		fallthrough
	case ".tar.sz":
		iface = &archive.TarSz{
			Tar: mytar,
		}

	case ".txz":
		fallthrough
	case ".tar.xz":
		iface = &archive.TarXz{
			Tar: mytar,
		}

	case ".zip":
		iface = archive.Zip{
			CompressionLevel:       compressionLevel,
			OverwriteExisting:      overwriteExisting,
			MkdirAll:               mkdirAll,
			SelectiveCompression:   selectiveCompression,
			ImplicitTopLevelFolder: implicitTopLevelFolder,
			ContinueOnError:        continueOnError,
		}

	default:
		archiveExt := filepath.Ext(archiveName)
		if archiveExt == "" {
			fatal("format missing (use file extension to specify archive/compression format)")
		} else {
			fatalf("unsupported format '%s'", archiveExt)
		}
	}

	var err error

	switch flag.Arg(0) {
	case "archive":
		a, ok := iface.(archive.Archiver)
		if !ok {
			fatalf("the archive command does not support the %s format", iface)
		}
		err = a.Archive(flag.Args()[2:], flag.Arg(1))

	case "unarchive":
		a, ok := iface.(archive.Unarchiver)
		if !ok {
			fatalf("the unarchive command does not support the %s format", iface)
		}
		err = a.Unarchive(flag.Arg(1), flag.Arg(2))

	case "extract":
		e, ok := iface.(archive.Extractor)
		if !ok {
			fatalf("the unarchive command does not support the %s format", iface)
		}
		err = e.Extract(flag.Arg(1), flag.Arg(2), flag.Arg(3))

	case "ls":
		w, ok := iface.(archive.Walker)
		if !ok {
			fatalf("the unarchive command does not support the %s format", iface)
		}

		var count int
		err = w.Walk(flag.Arg(1), func(f archive.File) error {
			count++
			switch h := f.Header.(type) {
			case *zip.FileHeader:
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

	default:
		fatalf("unrecognized command: %s", flag.Arg(0))
	}
	if err != nil {
		fatal(err)
	}
}

func fatal(v ...interface{}) {
	fmt.Fprintln(os.Stderr, v...)
	os.Exit(1)
}

func fatalf(s string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, s+"\n", v...)
	os.Exit(1)
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
	// TODO: add compression formats
}

const usage = `Usage: arc {archive|unarchive|extract|ls|help} <archive file> [files...]
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

  PASSWORD-PROTECTED RAR FILES
    Export the ARCHIVE_PASSWORD environment variable
    to be able to open password-protected RAR archives.	

  GLOBAL FLAG REFERENCE
    The following global flags may be used before the
    sub-command (some flags are format-specific):

`
