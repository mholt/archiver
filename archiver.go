package archiver

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Archiver represent a archive format
type Archiver interface {
	// Match checks supported files
	Match(filename string) bool
	// Make makes an archive file on disk.
	Make(destination string, sources []string) error
	// Open extracts an archive file on disk.
	Open(source, destination string) error
	// Write writes an archive to a Writer.
	Write(output io.Writer, sources []string) error
	// Read reads an archive from a Reader.
	Read(input io.Reader, destination string) error
}

// SupportedFormats contains all supported archive formats
var SupportedFormats = map[string]Archiver{}

// windowsReplacer replaces invalid characters in Windows filenames with underscore
var windowsReplacer = strings.NewReplacer(
	"<", "_",
	">", "_",
	":", "_",
	"\"", "_",
	"/", "_",
	"?", "_",
	"*", "_",
	"|", "_",
)

// replacedDirs tracks the replaced directory name for any given directory
var replacedDirs = make(map[string]string)

// RegisterFormat adds a supported archive format
func RegisterFormat(name string, format Archiver) {
	if _, ok := SupportedFormats[name]; ok {
		log.Printf("Format %s already exists, skip!\n", name)
		return
	}
	SupportedFormats[name] = format
}

// MatchingFormat returns the first archive format that matches
// the given file, or nil if there is no match
func MatchingFormat(fpath string) Archiver {
	for _, fmt := range SupportedFormats {
		if fmt.Match(fpath) {
			return fmt
		}
	}
	return nil
}

// replaceInvalidChars corrects invalid file and directory characters,
// and in the case of collision with an existing file or directory,
// adds an incrementing integer to the end
func replaceInvalidChars(name string, dir bool, rtype *strings.Replacer) string {
	rawName := name
	name = rtype.Replace(name)

	if rawName != name {
		// if a file belongs in a directory whose name was already replaced due
		// to invalid characters, replace the raw directory name with the
		// previously assigned replacement name
		if !dir {
			newDir, ok := replacedDirs[filepath.Dir(rawName)]

			if ok {
				name = path.Join(newDir, filepath.Base(name))
			}
		}

		// if the proposed replacement name doesn't yet exist, it is good to go
		if _, err := os.Stat(name); os.IsNotExist(err) {
			if dir {
				replacedDirs[rawName] = name
			}

			return name
		}

		// otherwise, loop until a unique sequence is found
		seq := 0

		for {
			seq++

			nameSeq := name + "-" + strconv.Itoa(seq)

			if _, err := os.Stat(nameSeq); err == nil {
				continue
			}

			if dir {
				replacedDirs[rawName] = nameSeq
			}

			return nameSeq
		}
	}

	return name
}

func writeNewFile(fpath string, in io.Reader, fm os.FileMode) error {
	if runtime.GOOS == "windows" {
		fpath = replaceInvalidChars(fpath, false, windowsReplacer)
	}

	err := os.MkdirAll(filepath.Dir(fpath), 0755)
	if err != nil {
		return fmt.Errorf("%s: making directory for file: %v", fpath, err)
	}

	out, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("%s: creating new file: %v", fpath, err)
	}
	defer out.Close()

	err = out.Chmod(fm)
	if err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("%s: changing file mode: %v", fpath, err)
	}

	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("%s: writing file: %v", fpath, err)
	}
	return nil
}

func writeNewSymbolicLink(fpath string, target string) error {
	if runtime.GOOS == "windows" {
		fpath = replaceInvalidChars(fpath, false, windowsReplacer)
		target = replaceInvalidChars(target, false, windowsReplacer)
	}

	err := os.MkdirAll(filepath.Dir(fpath), 0755)
	if err != nil {
		return fmt.Errorf("%s: making directory for file: %v", fpath, err)
	}

	err = os.Symlink(target, fpath)
	if err != nil {
		return fmt.Errorf("%s: making symbolic link for: %v", fpath, err)
	}

	return nil
}

func writeNewHardLink(fpath string, target string) error {
	if runtime.GOOS == "windows" {
		fpath = replaceInvalidChars(fpath, false, windowsReplacer)
		target = replaceInvalidChars(target, false, windowsReplacer)
	}

	err := os.MkdirAll(filepath.Dir(fpath), 0755)
	if err != nil {
		return fmt.Errorf("%s: making directory for file: %v", fpath, err)
	}

	err = os.Link(target, fpath)
	if err != nil {
		return fmt.Errorf("%s: making hard link for: %v", fpath, err)
	}

	return nil
}

func mkdir(dirPath string) error {
	if runtime.GOOS == "windows" {
		dirPath = replaceInvalidChars(dirPath, true, windowsReplacer)
	}

	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return fmt.Errorf("%s: making directory: %v", dirPath, err)
	}
	return nil
}
