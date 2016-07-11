archiver [![archiver GoDoc](https://img.shields.io/badge/reference-godoc-blue.svg?style=flat-square)](https://godoc.org/github.com/mholt/archiver) [![Linux Build Status](https://img.shields.io/travis/mholt/archiver.svg?style=flat-square&label=linux+build)](https://travis-ci.org/mholt/archiver) [![Windows Build Status](https://img.shields.io/appveyor/ci/mholt/archiver.svg?style=flat-square&label=windows+build)](https://ci.appveyor.com/project/mholt/archiver)
========

Package archiver makes it trivially easy to make and extract common archive formats such as .zip, and .tar.gz. Simply name the input and output file(s).

Files are put into the root of the archive; directories are recursively added.

The `archiver` command runs the same cross-platform and has no external dependencies (not even libc); powered by the Go standard library, [dsnet/compress](https://github.com/dsnet/compress), and [nwaples/rardecode](https://github.com/nwaples/rardecode). Enjoy!

Supported formats/extensions:

- .zip
- .tar.gz
- .tgz
- .tar.bz2
- .rar (open)


## Install

```bash
go get github.com/mholt/archiver/cmd/archiver
```

Or download binaries from the [releases](https://github.com/mholt/archiver/releases) page.


## Command Use

Make a new archive:

```bash
$ archiver make [archive name] [input files...]
```

(At least one input file is required.)

To extract an archive:

```bash
$ archiver open [archive name] [destination]
```

(The destination path is optional; default is current directory.)

The archive name must end with a supported file extension&mdash;this is how it knows what kind of archive to make. Run `archiver -h` for more help.


## Library Use

```go
import "github.com/mholt/archiver"
```

Create a .zip file:

```go
err := archiver.Zip("output.zip", []string{"file.txt", "folder"})
```

Extract a .zip file:

```go
err := archiver.Unzip("input.zip", "output_folder")
```

Working with other file formats is exactly the same, but with [their own functions](https://godoc.org/github.com/mholt/archiver).



## FAQ

#### Can I list a file in one folder to go into a different folder in the archive?

No. This works just like your OS would make an archive in the file explorer: organize your input files to mirror the structure you want in the archive.


#### Can it add files to an existing archive?

Nope. This is a simple tool; it just makes new archives or extracts existing ones.
