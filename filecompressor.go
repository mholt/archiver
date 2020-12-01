package archiver

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileCompressor can compress and decompress single files.
type FileCompressor struct {
	Compressor
	Decompressor

	// Whether to overwrite existing files when creating files.
	OverwriteExisting bool
}

// CompressFile reads the source file and compresses it to destination.
// The destination must have a matching extension.
func (fc FileCompressor) CompressFile(source, destination string) error {
	if err := fc.CheckExt(destination); err != nil {
		return err
	}
	if fc.Compressor == nil {
		return fmt.Errorf("no compressor specified")
	}
	if !fc.OverwriteExisting && fileExists(destination) {
		return fmt.Errorf("file exists: %s", destination)
	}

	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()

	return fc.Compress(in, out)
}

// DecompressFile reads the source file and decompresses it to destination.
func (fc FileCompressor) DecompressFile(source, destination string) error {
	if fc.Decompressor == nil {
		return fmt.Errorf("no decompressor specified")
	}

	if fileExists(destination) {
		if !fc.OverwriteExisting {
			return fmt.Errorf("file exists: %s", destination)
		}
		// Disallow symbolic link targets outside of the destination directory.
		info, err := os.Lstat(destination)
		if err != nil {
			return fmt.Errorf("failed to lstat destination file %s: %s", destination, err)
		} else if info.Mode()&os.ModeSymlink != 0 {
			linkDest, err := os.Readlink(destination)
			if err != nil {
				return fmt.Errorf("failed to read symbolic link %s: %s", destination, err)
			} else if !within(filepath.Dir(destination), linkDest) {
				return fmt.Errorf("symbolic link %s has a target outside of the destination directory: %s", destination, linkDest)
			}
		}
	}

	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()

	return fc.Decompress(in, out)
}
