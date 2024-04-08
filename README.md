# archiver [![Go Reference](https://pkg.go.dev/badge/github.com/mholt/archiver/v4.svg)](https://pkg.go.dev/github.com/mholt/archiver/v4) [![Ubuntu-latest](https://github.com/mholt/archiver/actions/workflows/ubuntu-latest.yml/badge.svg)](https://github.com/mholt/archiver/actions/workflows/ubuntu-latest.yml) [![Macos-latest](https://github.com/mholt/archiver/actions/workflows/macos-latest.yml/badge.svg)](https://github.com/mholt/archiver/actions/workflows/macos-latest.yml) [![Windows-latest](https://github.com/mholt/archiver/actions/workflows/windows-latest.yml/badge.svg)](https://github.com/mholt/archiver/actions/workflows/windows-latest.yml)

Introducing **Archiver 4.0** - a cross-platform, multi-format archive utility and Go library. A powerful and flexible library meets an elegant CLI in this generic replacement for several platform-specific or format-specific archive utilities.

**:warning: v4 is in ALPHA. The core library APIs work pretty well but the command has not been implemented yet, nor have most automated tests. If you need the `arc` command, stick with v3 for now.**

## Features

- Stream-oriented APIs
- Automatically identify archive and compression formats:
	- By file name
	- By header
- Traverse directories, archive files, and any other file uniformly as [`io/fs`](https://pkg.go.dev/io/fs) file systems:
	- [`DirFS`](https://pkg.go.dev/github.com/mholt/archiver/v4#DirFS)
	- [`FileFS`](https://pkg.go.dev/github.com/mholt/archiver/v4#FileFS)
	- [`ArchiveFS`](https://pkg.go.dev/github.com/mholt/archiver/v4#ArchiveFS)
- Compress and decompress files
- Create and extract archive files
- Walk or traverse into archive files
- Extract only specific files from archives
- Insert (append) into .tar and .zip archives
- Read from password-protected 7-Zip files
- Numerous archive and compression formats supported
- Extensible (add more formats just by registering them)
- Cross-platform, static binary
- Pure Go (no cgo)
- Multithreaded Gzip
- Adjust compression levels
- Automatically add compressed files to zip archives without re-compressing
- Open password-protected RAR archives

### Supported compression formats

- brotli (.br)
- bzip2 (.bz2)
- flate (.zip)
- gzip (.gz)
- lz4 (.lz4)
- lzip (.lz)
- snappy (.sz)
- xz (.xz)
- zlib (.zz)
- zstandard (.zst)

### Supported archive formats

- .zip
- .tar (including any compressed variants like .tar.gz)
- .rar (read-only)
- .7z (read-only)

Tar files can optionally be compressed using any compression format.

## Command use

Coming soon for v4. See [the last v3 docs](https://github.com/mholt/archiver/tree/v3.5.1).


## Library use

```bash
$ go get github.com/mholt/archiver/v4
```


### Create archive

Creating archives can be done entirely without needing a real disk or storage device since all you need is a list of [`File` structs](https://pkg.go.dev/github.com/mholt/archiver/v4#File) to pass in.

However, creating archives from files on disk is very common, so you can use the [`FilesFromDisk()` function](https://pkg.go.dev/github.com/mholt/archiver/v4#FilesFromDisk) to help you map filenames on disk to their paths in the archive. Then create and customize the format type.

In this example, we add 4 files and a directory (which includes its contents recursively) to a .tar.gz file:

```go
// map files on disk to their paths in the archive
files, err := archiver.FilesFromDisk(nil, map[string]string{
	"/path/on/disk/file1.txt": "file1.txt",
	"/path/on/disk/file2.txt": "subfolder/file2.txt",
	"/path/on/disk/file3.txt": "",              // put in root of archive as file3.txt
	"/path/on/disk/file4.txt": "subfolder/",    // put in subfolder as file4.txt
	"/path/on/disk/folder":    "Custom Folder", // contents added recursively
})
if err != nil {
	return err
}

// create the output file we'll write to
out, err := os.Create("example.tar.gz")
if err != nil {
	return err
}
defer out.Close()

// we can use the CompressedArchive type to gzip a tarball
// (compression is not required; you could use Tar directly)
format := archiver.CompressedArchive{
	Compression: archiver.Gz{},
	Archival:    archiver.Tar{},
}

// create the archive
err = format.Archive(context.Background(), out, files)
if err != nil {
	return err
}
```

The first parameter to `FilesFromDisk()` is an optional options struct, allowing you to customize how files are added.

### Extract archive

Extracting an archive, extracting _from_ an archive, and walking an archive are all the same function.

Simply use your format type (e.g. `Zip`) to call `Extract()`. You'll pass in a context (for cancellation), the input stream, the list of files you want out of the archive, and a callback function to handle each file. 

If you want all the files, pass in a nil list of file paths.

```go
// the type that will be used to read the input stream
format := archiver.Zip{}

// the list of files we want out of the archive; any
// directories will include all their contents unless
// we return fs.SkipDir from our handler
// (leave this nil to walk ALL files from the archive)
fileList := []string{"file1.txt", "subfolder"}

handler := func(ctx context.Context, f archiver.File) error {
	// do something with the file
	return nil
}

err := format.Extract(ctx, input, fileList, handler)
if err != nil {
	return err
}
```

### Identifying formats

Have an input stream with unknown contents? No problem, archiver can identify it for you. It will try matching based on filename and/or the header (which peeks at the stream):

```go
format, input, err := archiver.Identify("filename.tar.zst", input)
if err != nil {
	return err
}
// you can now type-assert format to whatever you need;
// be sure to use returned stream to re-read consumed bytes during Identify()

// want to extract something?
if ex, ok := format.(archiver.Extractor); ok {
	// ... proceed to extract
}

// or maybe it's compressed and you want to decompress it?
if decom, ok := format.(archiver.Decompressor); ok {
	rc, err := decom.OpenReader(unknownFile)
	if err != nil {
		return err
	}
	defer rc.Close()

	// read from rc to get decompressed data
}
```

`Identify()` works by reading an arbitrary number of bytes from the beginning of the stream (just enough to check for file headers). It buffers them and returns a new reader that lets you re-read them anew.

### Virtual file systems

This is my favorite feature.

Let's say you have a file. It could be a real directory on disk, an archive, a compressed archive, or any other regular file. You don't really care; you just want to use it uniformly no matter what it is.

Use archiver to simply create a file system:

```go
// filename could be:
// - a folder ("/home/you/Desktop")
// - an archive ("example.zip")
// - a compressed archive ("example.tar.gz")
// - a regular file ("example.txt")
// - a compressed regular file ("example.txt.gz")
fsys, err := archiver.FileSystem(filename)
if err != nil {
	return err
}
```

This is a fully-featured `fs.FS`, so you can open files and read directories, no matter what kind of file the input was.

For example, to open a specific file:

```go
f, err := fsys.Open("file")
if err != nil {
	return err
}
defer f.Close()
```

If you opened a regular file, you can read from it. If it's a compressed file, reads are automatically decompressed.

If you opened a directory, you can list its contents:

```go
if dir, ok := f.(fs.ReadDirFile); ok {
	// 0 gets all entries, but you can pass > 0 to paginate
	entries, err := dir.ReadDir(0)
	if err != nil {
		return err
	}
	for _, e := range entries {
		fmt.Println(e.Name())
	}
}
```

Or get a directory listing this way:

```go
entries, err := fsys.ReadDir("Playlists")
if err != nil {
	return err
}
for _, e := range entries {
	fmt.Println(e.Name())
}
```

Or maybe you want to walk all or part of the file system, but skip a folder named `.git`:

```go
err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if path == ".git" {
		return fs.SkipDir
	}
	fmt.Println("Walking:", path, "Dir?", d.IsDir())
	return nil
})
if err != nil {
	return err
}
```

#### Use with `http.FileServer`

It can be used with http.FileServer to browse archives and directories in a browser. However, due to how http.FileServer works, don't directly use http.FileServer with compressed files; instead wrap it like following:

```go
fileServer := http.FileServer(http.FS(archiveFS))
http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
	// disable range request
	writer.Header().Set("Accept-Ranges", "none")
	request.Header.Del("Range")
	
	// disable content-type sniffing
	ctype := mime.TypeByExtension(filepath.Ext(request.URL.Path))
	writer.Header()["Content-Type"] = nil
	if ctype != "" {
		writer.Header().Set("Content-Type", ctype)
	}
	fileServer.ServeHTTP(writer, request)
})
```

http.FileServer will try to sniff the Content-Type by default if it can't be inferred from file name. To do this, the http package will try to read from the file and then Seek back to file start, which the libray can't achieve currently. The same goes with Range requests. Seeking in archives is not currently supported by archiver due to limitations in dependencies.

If content-type is desirable, you can [register it](https://pkg.go.dev/mime#AddExtensionType) yourself.

### Compress data

Compression formats let you open writers to compress data:

```go
// wrap underlying writer w
compressor, err := archiver.Zstd{}.OpenWriter(w)
if err != nil {
	return err
}
defer compressor.Close()

// writes to compressor will be compressed
```

### Decompress data

Similarly, compression formats let you open readers to decompress data:

```go
// wrap underlying reader r
decompressor, err := archiver.Brotli{}.OpenReader(r)
if err != nil {
	return err
}
defer decompressor.Close()

// reads from decompressor will be decompressed
```

### Append to tarball and zip archives

Tar and Zip archives can be appended to without creating a whole new archive by calling `Insert()` on a tar or zip stream. However, for tarballs, this requires that the tarball is not compressed (due to complexities with modifying compression dictionaries).

Here is an example that appends a file to a tarball on disk:

```go
tarball, err := os.OpenFile("example.tar", os.O_RDWR, 0644)
if err != nil {
	return err
}
defer tarball.Close()

// prepare a text file for the root of the archive
files, err := archiver.FilesFromDisk(nil, map[string]string{
	"/home/you/lastminute.txt": "",
})

err := archiver.Tar{}.Insert(context.Background(), tarball, files)
if err != nil {
	return err
}
```

The code is similar for inserting into a Zip archive, except you'll call `Insert()` on the `Zip` type instead.

