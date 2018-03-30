package archiver

import (
	"fmt"
	"github.com/go-ini/ini"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
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

func writeNewFile(fpath string, in io.Reader, fm os.FileMode) error {
	is_can_create := filterFolder(fpath)
	if is_can_create == true {
		return nil
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
	is_can_create := filterFolder(fpath)
	if is_can_create == true {
		return nil
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
	is_can_create := filterFolder(fpath)
	if is_can_create == true {
		return nil
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
	is_can_create := filterFolder(dirPath)
	if is_can_create == true {
		return nil
	}

	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return fmt.Errorf("%s: making directory: %v", dirPath, err)
	}
	return nil
}
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
func filterFolder(path string) bool {
	str := os.ExpandEnv("${GOPATH}/src/github.com/bingyangzeng/archiver/conf/app.ini")
	is_exists,err:= PathExists(str)
	inipath := "./conf/app.ini"
	if is_exists{
		inipath = str 
	}
	cfg,err := ini.Load(inipath)
	if err != nil {
		fmt.Println(err)
		return false
	}
	
	filter := cfg.Section("filters").Key("path").String()
	filter_slice := strings.Fields(filter)
	is_need_filter := false
	if len(filter_slice) > 0 {
		for _, v := range filter_slice {
			ishave := strings.Contains(path, v)
			if ishave == true {
				log.Printf("dir %s is filter %s , skip!\n",v, path)
				is_need_filter = true
				break
			}
		}
	}
	return is_need_filter
}
