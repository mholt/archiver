archiver [![archiver GoDoc](https://img.shields.io/badge/reference-godoc-blue.svg?style=flat-square)](https://godoc.org/github.com/mholt/archiver) [![Linux Build Status](https://img.shields.io/travis/mholt/archiver.svg?style=flat-square&label=linux+build)](https://travis-ci.org/mholt/archiver) [![Windows Build Status](https://img.shields.io/appveyor/ci/mholt/archiver.svg?style=flat-square&label=windows+build)](https://ci.appveyor.com/project/mholt/archiver)
========

Package archiver makes it trivially easy to make and extract common archive formats such as .zip, and .tar.gz. Simply name the input and output file(s).

Files are put into the root of the archive; directories are recursively added, preserving structure.

The `archiver` command runs the same cross-platform and has no external dependencies (not even libc); powered by the Go standard library, [dsnet/compress](https://github.com/dsnet/compress), [nwaples/rardecode](https://github.com/nwaples/rardecode), and [ulikunitz/xz](https://github.com/ulikunitz/xz). Enjoy!

Supported formats/extensions:

- .zip
- .tar
- .tar.gz & .tgz
- .tar.bz2 & .tbz2
- .tar.xz & .txz
- .tar.lz4 & .tlz4
- .tar.sz & .tsz
- .rar (open only)


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
err := archiver.Zip.Make("output.zip", []string{"file.txt", "folder"})
```

Extract a .zip file:

```go
err := archiver.Zip.Open("input.zip", "output_folder")
```

Working with other file formats is exactly the same, but with [their own Archiver implementations](https://godoc.org/github.com/mholt/archiver#Archiver).



## FAQ

#### Can I list a file in one folder to go into a different folder in the archive?

No. This works just like your OS would make an archive in the file explorer: organize your input files to mirror the structure you want in the archive.


#### Can it add files to an existing archive?

Nope. This is a simple tool; it just makes new archives or extracts existing ones.


## Project Values

This project has a few principle-based goals that guide its development:

- **Do one thing really well.** That is creating and opening archive files. It is not meant to be a replacement for specific archive format tools like tar, zip, etc. that have lots of features and customizability. (Some customizability is OK, but not to the extent that it becomes complicated or error-prone.)

- **Have good tests.** Changes should be covered by tests.

- **Limit dependencies.** Keep the package lightweight.

- **Pure Go.** This means no cgo or other external/system dependencies. This package should be able to stand on its own and cross-compile easily to any platform.

- **Idiomatic Go.** Keep interfaces small, variable names semantic, vet shows no errors, the linter is generally quiet, etc.

- **Be elegant.** This package should be elegant to use and its code should be elegant when reading and testing. If it doesn't feel good, fix it up.

- **Well-documented.** Use comments prudently; explain why non-obvious code is necessary (and use tests to enforce it). Keep the docs updated, and have examples where helpful.

- **Keep it efficient.** This often means keep it simple. Fast code is valuable.

- **Consensus.** Contributions should ideally be approved by multiple reviewers before being merged. Generally, avoid merging multi-chunk changes that do not go through at least one or two iterations/reviews. Except for trivial changes, PRs are seldom ready to merge right away.

- **Have fun contributing.** Coding is awesome!

We welcome contributions and appreciate your efforts! However, please open issues to discuss any changes before spending the time preparing a pull request. This will save time, reduce frustration, and help coordinate the work. Thank you!
