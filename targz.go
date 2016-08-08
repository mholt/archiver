package archiver

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// TarGz creates a .tar.gz file at targzPath containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func TarGz(targzPath string, filePaths []string) error {
	out, err := os.Create(targzPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", targzPath, err)
	}
	defer out.Close()

	gzWriter := gzip.NewWriter(out)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	return tarball(filePaths, tarWriter, targzPath)
}

// tarball writes all files listed in filePaths into tarWriter, which is
// writing into a file located at dest.
func tarball(filePaths []string, tarWriter *tar.Writer, dest string) error {
	for _, fpath := range filePaths {
		err := tarFile(tarWriter, fpath, dest)
		if err != nil {
			return err
		}
	}
	return nil
}

// tarFile writes the file at source into tarWriter. It does so
// recursively for directories.
func tarFile(tarWriter *tar.Writer, source, dest string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("%s: stat: %v", source, err)
	}

	var baseDir string
	if sourceInfo.IsDir() {
		baseDir = filepath.Base(source)
	}

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking to %s: %v", path, err)
		}

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return fmt.Errorf("%s: making header: %v", path, err)
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}

		if header.Name == dest {
			// our new tar file is inside the directory being archived; skip it
			return nil
		}

		if info.IsDir() {
			header.Name += "/"
		}

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return fmt.Errorf("%s: writing header: %v", path, err)
		}

		if info.IsDir() {
			return nil
		}

		if header.Typeflag == tar.TypeReg {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("%s: open: %v", path, err)
			}
			defer file.Close()

			_, err = io.CopyN(tarWriter, file, info.Size())
			if err != nil && err != io.EOF {
				return fmt.Errorf("%s: copying contents: %v", path, err)
			}
		}
		return nil
	})
}

// UntarGz untars source and decompresses the contents into destination.
func UntarGz(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%s: create new gzip reader: %v", source, err)
	}
	defer gzr.Close()

	return untar(tar.NewReader(gzr), destination)
}

// untar un-tarballs the contents of tr into destination.
func untar(tr *tar.Reader, destination string) error {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if err := untarFile(tr, header, destination); err != nil {
			return err
		}
	}
	return nil
}

// untarFile untars a single file from tr with header header into destination.
func untarFile(tr *tar.Reader, header *tar.Header, destination string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return mkdir(filepath.Join(destination, header.Name))
	case tar.TypeReg, tar.TypeRegA:
		return writeNewFile(filepath.Join(destination, header.Name), tr, header.FileInfo().Mode())
	case tar.TypeSymlink:
		return writeNewSymbolicLink(filepath.Join(destination, header.Name), header.Linkname)
	default:
		return fmt.Errorf("%s: unknown type flag: %c", header.Name, header.Typeflag)
	}
}
