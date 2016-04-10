archiver [![archiver GoDoc](https://img.shields.io/badge/reference-godoc-blue.svg?style=flat-square)](https://godoc.org/github.com/mholt/archiver)
========

Package archiver makes it trivially easy to make and extract .zip and .tar.gz files. Simply give the input and output file(s).

Files are put into the root of the archive; directories are recursively added.


## Install

```bash
go get github.com/mholt/archiver
```


## Use

Create a .zip file:

```go
err := archiver.Zip("output.zip", []string{"file.txt", "folder"})
```

Create a .tar.gz file:

```go
err := archiver.TarGz("output.tar.gz", []string{"file.txt",	"folder"})
```

Extract a .zip file:

```go
err := archiver.Unzip("input.zip", "output_folder")
```

Extract a .tar.gz file:

```go
err := archiver.UntarGz("input.tar.gz", "output_folder")
```


## FAQ

#### Can I list a file to go in a different folder in the archive?

No, because I don't need it to do that. Just structure your source files to mirror the structure in the archive, like you would normally do when you make an archive using your OS.
