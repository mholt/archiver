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
	dirPermissions  = 0o700 // Directory permissions
	filePermissions = 0o600 // File permissions
)

// securePath ensures the path is safely relative to the target directory.
func securePath(basePath, relativePath string) (string, error) {
	// Clean and ensure the relative path does not start with an absolute marker
	relativePath = filepath.Clean("/" + relativePath)                         // Normalize path with a leading slash
	relativePath = strings.TrimPrefix(relativePath, string(os.PathSeparator)) // Remove leading separator

	// Join the cleaned relative path with the basePath
	dstPath := filepath.Join(basePath, relativePath)

	// Ensure the final destination path is within the basePath
	if !strings.HasPrefix(filepath.Clean(dstPath)+string(os.PathSeparator), filepath.Clean(basePath)+string(os.PathSeparator)) {
		return "", fmt.Errorf("illegal file path: %s", dstPath)
	}
	return dstPath, nil
}

// createDir creates a directory with predefined permissions.
func createDir(path string) error {
	if err := os.MkdirAll(path, dirPermissions); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	return nil
}

// handleFile handles the extraction of a file from the archive.
func handleFile(f archiver.File, dst string) error {
	// Log the name of the file being processed
	log.Printf("Handling file: %s", f.NameInArchive)

	// Validate and construct the destination path
	dstPath, err := securePath(dst, f.NameInArchive)
	if err != nil {
		return err
	}

	// This log should now be visible if `handleFile` is called
	log.Printf("Extracting file to: %s", dstPath)

	// Ensure the parent directory exists
	if err := createDir(filepath.Dir(dstPath)); err != nil {
		return err
	}

	// Check if the file is a directory
	if f.IsDir() {
		// If it's a directory, ensure it exists
		if err := createDir(dstPath); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		log.Printf("Successfully created directory: %s", dstPath)
		return nil
	}

	// Open the file for reading
	reader, err := f.Open()
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer reader.Close()

	// Create the destination file
	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY, filePermissions)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer dstFile.Close()

	// Copy the file contents
	if _, err := io.Copy(dstFile, reader); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	log.Printf("Successfully extracted file: %s", dstPath)
	return nil
}

// Unarchive unarchives a tarball to a directory using the official extraction method.
func Unarchive(tarball, dst string) error {
	f, err := os.Open(tarball)
	if err != nil {
		return fmt.Errorf("open tarball %s: %w", tarball, err)
	}
	// Identify the format and input stream for the archive
	format, input, err := archiver.Identify(tarball, f)
	if err != nil {
		return fmt.Errorf("identify format: %w", err)
	}

	// Check if the format supports extraction
	extractor, ok := format.(archiver.Extractor)
	if !ok {
		return fmt.Errorf("unsupported format for extraction")
	}

	// Ensure the destination directory exists
	if err := createDir(dst); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}
	log.Printf("Destination directory created or already exists: %s", dst)

	// Extract files using the official handler
	handler := func(ctx context.Context, f archiver.File) error {
		log.Printf("Processing file: %s", f.NameInArchive)
		return handleFile(f, dst)
	}

	// Use the extractor to process all files in the archive
	if err := extractor.Extract(context.Background(), input, nil, handler); err != nil {
		return fmt.Errorf("extracting files: %w", err)
	}

	log.Printf("Unarchiving completed successfully.")
	return nil
}

func main() {
	tarball := flag.String("f", "", "Archive to extract")
	dst := flag.String("d", "", "Destination directory")
	flag.Parse()

	// unarchive
	err := Unarchive(*tarball, *dst)
	if err != nil {
		log.Fatal(err)
	}
}
