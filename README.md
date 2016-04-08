archiver
========

Package archiver makes it trivially easy to make .zip and .tar.gz files. Simply give the output filename and a list of files/folders you want included in the archive.

The goal of this package is to make it as easy for Go programmers to make archives as it is for a computer user who just right-clicks and chooses something like "Compress".

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


## FAQ

#### Can it unzip and untar?

No, because I haven't needed that yet. But if there's enough demand, we can add it. Pull requests welcome! **Remember: a pull request, with test, is best.**


#### Can I list a file to go in a different folder in the archive?

No, because I didn't need it to do that. Just structure your source files to mirror the structure in the archive, like you would normally do when you make an archive using your OS.

