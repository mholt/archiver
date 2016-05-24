archiver [![archiver GoDoc](https://img.shields.io/badge/reference-godoc-blue.svg?style=flat-square)](https://godoc.org/github.com/mholt/archiver) [![Linux Build Status](https://img.shields.io/travis/mholt/archiver.svg?style=flat-square&label=linux+build)](https://travis-ci.org/mholt/archiver) [![Windows Build Status](https://img.shields.io/appveyor/ci/mholt/archiver.svg?style=flat-square&label=windows+build)](https://ci.appveyor.com/project/mholt/archiver)
========

Package archiver makes it trivially easy to make and extract .zip and .tar.gz files. Simply give the input and output file(s).

Files are put into the root of the archive; directories are recursively added.


## Install

```bash
go get github.com/mholt/archiver
```


## Library Use

Create a .zip file:

```go
err := archiver.Zip("output.zip", []string{"file.txt", "folder"})
```

Extract a .zip file:

```go
err := archiver.Unzip("input.zip", "output_folder")
```

Create a .tar.gz file:

```go
err := archiver.TarGz("output.tar.gz", []string{"file.txt",	"folder"})
```

Extract a .tar.gz file:

```go
err := archiver.UntarGz("input.tar.gz", "output_folder")
```


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

The archive name must end with a supported file extension like .zip or .tar.gz&mdash;this is how it knows what kind of archive to make.



## FAQ

#### Can I list a file to go in a different folder in the archive?

No, because I didn't need it to do that. Just structure your source files to mirror the structure in the archive, like you would normally do when you make an archive using your OS.


#### Can it add files to an existing archive?

Nope. It just makes new archives or extracts existing ones.
