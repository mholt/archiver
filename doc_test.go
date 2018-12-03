package archiver

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
)

// The simplest use of this package: create an archive file
// from a list of filenames. This is the recommended way to
// do so using a default configuration, as it guarantees
// the file format matches the file extension, because the
// format to write is determined by the given extension.
func ExampleArchive() {
	// any files in this list are added
	// to the top level of the archive;
	// directories are recursively added
	files := []string{
		"index.html",
		"photo.jpg",
		"blog", // directory
		"/home/website/copyright.txt",
	}

	// archive format is determined by file extension
	err := Archive(files, "blog_site.zip")
	if err != nil {
		log.Fatal(err)
	}
}

// The simplest use of this package: extract all of an archive's
// contents to a folder on disk using the default configuration.
// The archive format is determined automatically.
func ExampleUnarchive() {
	err := Unarchive("blog_site.zip", "extracted/mysite")
	if err != nil {
		log.Fatal(err)
	}
}

// In this example, the DefaultZip is being customized so that
// all calls to its methods will use that configuration.
func ExampleZip_default() {
	DefaultZip.OverwriteExisting = true
	DefaultZip.ImplicitTopLevelFolder = true
	// any subsequent use of DefaultZip uses
	// this modified configuration
}

// Here we create our own instance of the Zip format. No need
// to use the constructor function (NewZip) or the default
// instance (DefaultZip) if we do not want to. Instantiating
// the type like this allows us to easily be very explicit
// about our configuration.
func ExampleZip_custom() {
	z := &Zip{
		CompressionLevel:       3,
		OverwriteExisting:      false,
		MkdirAll:               true,
		SelectiveCompression:   true,
		ImplicitTopLevelFolder: true,
		ContinueOnError:        false,
	}
	// z is now ready to use for whatever (this is a dumb example)
	fmt.Println(z.CheckExt("test.zip"))
}

// Much like the package-level Archive function, this creates an
// archive using the configuration of the Zip instance it is called
// on. The output filename must match the format's recognized file
// extension(s).
func ExampleZip_Archive() {
	err := DefaultZip.Archive([]string{"..."}, "example.zip")
	if err != nil {
		log.Fatal(err)
	}
}

// It's easy to list the items in an archive. This example
// prints the name and size of each file in the archive. Like
// other top-level functions in this package, the format is
// inferred automatically for you.
func ExampleWalk() {
	err := Walk("example.tar.gz", func(f File) error {
		fmt.Println(f.Name(), f.Size())
		// you could also read the contents; f is an io.Reader!
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

// This example extracts target.txt from inside example.rar
// and puts it into a folder on disk called output/dir.
func ExampleExtract() {
	err := Extract("example.rar", "target.txt", "output/dir")
	if err != nil {
		log.Fatal(err)
	}
}

// This example demonstrates how to read an
// archive in a streaming fashion. The idea
// is that you can stream the bytes of an
// archive from a stream, regardless of
// whether it is an actual file on disk.
// This means that you can read a huge
// archive file-by-file rather than having
// to store it all on disk first. In this
// example, we read a hypothetical archive
// from a (fake) HTTP request body and
// print its file names and sizes. The
// files can be read, of course, but they
// do not have to be.
func ExampleZip_streamingRead() {
	// for the sake of the example compiling, pretend we have an HTTP request
	req := new(http.Request)
	contentLen, err := strconv.Atoi(req.Header.Get("Content-Length"))
	if err != nil {
		log.Fatal(err)
	}

	// the Zip format requires knowing the length of the stream,
	// but other formats don't generally require it, so it
	// could be left as 0 when using those
	err = DefaultZip.Open(req.Body, int64(contentLen))
	if err != nil {
		log.Fatal(err)
	}
	defer DefaultZip.Close()

	// Note that DefaultZip now contains some state that
	// is critical to reading the stream until it is closed,
	// so do not reuse it until then.

	// iterate each file in the archive until EOF
	for {
		f, err := DefaultZip.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		// f is an io.ReadCloser, so you can read its contents
		// if you wish; or you can access its header info through
		// f.Header or the embedded os.FileInfo
		fmt.Println("File name:", f.Name(), "File size:", f.Size())

		// be sure to close f before moving on!!
		err = f.Close()
		if err != nil {
			log.Fatal(err)
		}
	}
}

// This example demonstrates how to write an
// archive in a streaming fashion. The idea
// is that you can stream the bytes of a new
// archive that is created on-the-fly from
// generic streams. Those streams could be
// actual files on disk, or they could be over
// a network, or standard output, or any other
// io.Reader/io.Writer. This example only adds
// one file to the archive and writes the
// resulting archive to standard output, but you
// could add as many files as needed with a loop.
func ExampleZip_streamingWrite() {
	err := DefaultZip.Create(os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
	defer DefaultZip.Close()

	// Note that DefaultZip now contains state
	// critical to a successful write until it
	// is closed, so don't reuse it for anything
	// else until then.

	// At this point, you can open an actual file
	// to add to the archive, or the "file" could
	// come from any io.ReadCloser stream. If you
	// only have an io.Reader, you can use
	// ReadFakeCloser to make it into an
	// io.ReadCloser.

	// The next part is a little tricky if you
	// don't have an actual file because you will
	// need an os.FileInfo. Fortunately, that's an
	// interface! So go ahead and implement it in
	// whatever way makes the most sense to you.
	// You'll also need to give the file a name
	// for within the archive. In this example,
	// we'll open a real file.

	file, err := os.Open("foo.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatal(err)
	}

	err = DefaultZip.Write(File{
		FileInfo: FileInfo{
			FileInfo:   fileInfo,
			CustomName: "name/in/archive.txt",
		},
		ReadCloser: file, // does not have to be an actual file
	})
	if err != nil {
		log.Fatal(err)
	}
}

// This example compresses a standard tar file into a tar.gz file.
// Compression formats are selected by file extension.
func ExampleCompressFile() {
	err := CompressFile("example.tar", "example.tar.gz")
	if err != nil {
		log.Fatal(err)
	}
}

// This example changes the default configuration for
// the Gz compression format.
func ExampleCompressFile_custom() {
	DefaultGz.CompressionLevel = 5
	// any calls to DefaultGz now use the modified configuration
}

// This example creates a new Gz instance and
// uses it to compress a stream, writing to
// another stream. This is sometimes preferable
// over modifying the DefaultGz.
func ExampleGz_Compress_custom() {
	gz := &Gz{CompressionLevel: 5}
	err := gz.Compress(os.Stdin, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
}

// This example decompresses a gzipped tarball and writes
// it to an adjacent file.
func ExampleDecompressFile() {
	err := DecompressFile("example.tar.gz", "example.tar")
	if err != nil {
		log.Fatal(err)
	}
}
