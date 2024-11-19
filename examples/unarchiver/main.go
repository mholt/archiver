package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v4"
)

const (
	dirPermissions  = 0o700 // Default directory permissions
	filePermissions = 0o600 // Default file permissions
)

// securePath ensures the path is safely relative to the target directory.
func securePath(basePath, relativePath string) (string, error) {
	relativePath = filepath.Clean("/" + relativePath)                         // Normalize path with a leading slash
	relativePath = strings.TrimPrefix(relativePath, string(os.PathSeparator)) // Remove leading separator

	dstPath := filepath.Join(basePath, relativePath)

	if !strings.HasPrefix(filepath.Clean(dstPath)+string(os.PathSeparator), filepath.Clean(basePath)+string(os.PathSeparator)) {
		return "", fmt.Errorf("illegal file path: %s", dstPath)
	}
	return dstPath, nil
}

// createDirWithPermissions creates a directory with specified permissions.
func createDirWithPermissions(path string, mode os.FileMode) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	return nil
}

// setPermissions applies permissions to a file or directory.
func setPermissions(path string, mode os.FileMode) error {
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	return nil
}

// handleFile handles the extraction of a file from the archive.
func handleFile(f archiver.File, dst string) error {
	log.Printf("Handling file: %s", f.NameInArchive)

	// Validate and construct the destination path
	dstPath, pathErr := securePath(dst, f.NameInArchive)
	if pathErr != nil {
		return pathErr
	}

	// Ensure the parent directory exists
	parentDir := filepath.Dir(dstPath)
	if dirErr := createDirWithPermissions(parentDir, dirPermissions); dirErr != nil {
		return dirErr
	}

	// Handle directories
	if f.IsDir() {
		// Create the directory with permissions from the archive
		if dirErr := createDirWithPermissions(dstPath, f.Mode()); dirErr != nil {
			return fmt.Errorf("creating directory: %w", dirErr)
		}
		log.Printf("Successfully created directory: %s", dstPath)
		return nil
	}

	// Handle symlinks
	if f.LinkTarget != "" {
		targetPath, linkErr := securePath(dst, f.LinkTarget)
		if linkErr != nil {
			return fmt.Errorf("invalid symlink target: %w", linkErr)
		}
		if linkErr := os.Symlink(targetPath, dstPath); linkErr != nil {
			return fmt.Errorf("create symlink: %w", linkErr)
		}
		log.Printf("Successfully created symlink: %s -> %s", dstPath, targetPath)
		return nil
	}

	// Check and handle parent directory permissions
	originalMode, statErr := os.Stat(parentDir)
	if statErr != nil {
		return fmt.Errorf("stat parent directory: %w", statErr)
	}

	// If parent directory is read-only, temporarily make it writable
	if originalMode.Mode().Perm()&0200 == 0 {
		log.Printf("Parent directory is read-only, temporarily making it writable: %s", parentDir)
		if chmodErr := os.Chmod(parentDir, originalMode.Mode()|0200); chmodErr != nil {
			return fmt.Errorf("chmod parent directory: %w", chmodErr)
		}
		defer func() {
			// Restore the original permissions after writing
			if chmodErr := os.Chmod(parentDir, originalMode.Mode()); chmodErr != nil {
				log.Printf("Failed to restore original permissions for %s: %v", parentDir, chmodErr)
			}
		}()
	}

	// Handle regular files
	reader, openErr := f.Open()
	if openErr != nil {
		return fmt.Errorf("open file: %w", openErr)
	}
	defer reader.Close()

	dstFile, createErr := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY, f.Mode())
	if createErr != nil {
		return fmt.Errorf("create file: %w", createErr)
	}
	defer dstFile.Close()

	if _, copyErr := io.Copy(dstFile, reader); copyErr != nil {
		return fmt.Errorf("copy: %w", copyErr)
	}
	log.Printf("Successfully extracted file: %s", dstPath)
	return nil
}

// Unarchive unarchives a tarball to a directory using the official extraction method.
func Unarchive(tarball, dst string) error {
	archiveFile, openErr := os.Open(tarball)
	if openErr != nil {
		return fmt.Errorf("open tarball %s: %w", tarball, openErr)
	}
	defer archiveFile.Close()

	format, input, identifyErr := archiver.Identify(tarball, archiveFile)
	if identifyErr != nil {
		return fmt.Errorf("identify format: %w", identifyErr)
	}

	extractor, ok := format.(archiver.Extractor)
	if !ok {
		return fmt.Errorf("unsupported format for extraction")
	}

	if dirErr := createDirWithPermissions(dst, dirPermissions); dirErr != nil {
		return fmt.Errorf("creating destination directory: %w", dirErr)
	}
	log.Printf("Destination directory created or already exists: %s", dst)

	handler := func(ctx context.Context, f archiver.File) error {
		return handleFile(f, dst)
	}

	if extractErr := extractor.Extract(context.Background(), input, nil, handler); extractErr != nil {
		return fmt.Errorf("extracting files: %w", extractErr)
	}

	log.Printf("Unarchiving completed successfully.")
	return nil
}

func main() {
	tarball := flag.String("f", "", "Archive to extract")
	dst := flag.String("d", "", "Destination directory")
	flag.Parse()

	if *tarball == "" || *dst == "" {
		log.Fatal("Both archive and destination must be specified")
	}

	err := Unarchive(*tarball, *dst)
	if err != nil {
		log.Fatal(err)
	}
}
