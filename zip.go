// Package archiver makes it super easy to create .zip and .tar.gz archives.
package archiver

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Zip creates a .zip file in the location zipPath containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
//
// Files with an extension for formats that are already
// compressed will be stored only, not compressed.
func Zip(zipPath string, filePaths []string) error {
	out, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", zipPath, err)
	}
	defer out.Close()

	w := zip.NewWriter(out)
	for _, fpath := range filePaths {
		err = zipFile(w, fpath)
		if err != nil {
			w.Close()
			return err
		}
	}

	return w.Close()
}

func zipFile(w *zip.Writer, source string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("%s: stat: %v", source, err)
	}

	var baseDir string
	if sourceInfo.IsDir() {
		baseDir = filepath.Base(source)
	}

	return filepath.Walk(source, func(fpath string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walking to %s: %v", fpath, err)
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("%s: getting header: %v", fpath, err)
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(fpath, source))
		}

		if info.IsDir() {
			header.Name += "/"
			header.Method = zip.Store
		} else {
			ext := strings.ToLower(path.Ext(header.Name))
			if _, ok := CompressedFormats[ext]; ok {
				header.Method = zip.Store
			} else {
				header.Method = zip.Deflate
			}
		}

		writer, err := w.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("%s: making header: %v", fpath, err)
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(fpath)
		if err != nil {
			return fmt.Errorf("%s: opening: %v", fpath, err)
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		if err != nil {
			return fmt.Errorf("%s: copying contents: %v", fpath, err)
		}

		return nil
	})
}

// CompressedFormats is a set of lowercased file extensions
// for file formats that are typically already compressed.
// Compressing already-compressed files often results in
// a larger file. This list is not an exhaustive.
var CompressedFormats = map[string]struct{}{
	".7z":   struct{}{},
	".avi":  struct{}{},
	".bz2":  struct{}{},
	".gif":  struct{}{},
	".gz":   struct{}{},
	".jpeg": struct{}{},
	".jpg":  struct{}{},
	".lz":   struct{}{},
	".lzma": struct{}{},
	".mov":  struct{}{},
	".mp3":  struct{}{},
	".mp4":  struct{}{},
	".mpeg": struct{}{},
	".mpg":  struct{}{},
	".png":  struct{}{},
	".rar":  struct{}{},
	".xz":   struct{}{},
	".zip":  struct{}{},
	".zipx": struct{}{},
}
