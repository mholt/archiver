// +build gofuzz

package archiver

import (
	"io/ioutil"
)

func FuzzZip(input []byte) int {
	err := ioutil.WriteFile("fuzz/archive.zip", input, 0644)
	if err != nil {
		return 0
	}
	err = Walk("fuzz/archive.zip", func(file File) error {
		return nil
	})
	if err != nil {
		return 0
	}
	err = Unarchive("fuzz/archive.zip", "fuzz/contents")
	if err != nil {
		return 0
	}
	return 1
}

func FuzzLz4(input []byte) int {
	err := ioutil.WriteFile("fuzz/compressed.lz4", input, 0644)
	if err != nil {
		return 0
	}
	err = DecompressFile("fuzz/compressed.lz4", "fuzz/decompressed")
	if err != nil {
		return 0
	}
	return 1
}
