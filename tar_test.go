package archiver

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
)

func createTestFile(testDir, filePath string, fileContent []byte) (err error) {
	if strings.Contains(filePath, "/") {
		// Need to create subfolder(s) before creating file
		if e := os.MkdirAll(path.Join(testDir, path.Dir(filePath)), 0777); e != nil {
			err = e
			return
		}
	}

	file, err := os.Create(path.Join(testDir, filePath))
	if err != nil {
		return
	}

	_, err = file.Write(fileContent)
	return
}

func createRandomBytes(t *testing.T) (bytes []byte) {
	bytes = make([]byte, 100)
	_, err := rand.Read(bytes)
	if err != nil {
		if err != nil {
			t.Fatalf("Making random byte array: %v", err)
		}
	}
	return
}

func TestTar(t *testing.T) {
	inputDir, err := ioutil.TempDir("", "inputFolder")
	if err != nil {
		t.Fatalf("Making temporary directory: %v", err)
	}
	defer os.RemoveAll(inputDir)

	fileContent1 := createRandomBytes(t)
	filePath1 := "file1"

	fileContent2 := createRandomBytes(t)
	filePath2 := "subpath/file2"

	fileContent3 := createRandomBytes(t)
	filePath3 := "subpath2/subpath3/file3"

	for _, td := range []struct {
		filePath string
		fileData []byte
	}{
		{filePath1, fileContent1},
		{filePath2, fileContent2},
		{filePath3, fileContent3},
	} {
		if err := createTestFile(inputDir, td.filePath, td.fileData); err != nil {
			t.Fatalf("Making temporary file %s: %v", td.filePath, err)
		}
	}


	t.Run("Create tar and untar it to a new location using Archive/Unarchive", func(t *testing.T) {
		filesInInputDir, err := ioutil.ReadDir(inputDir)
		if err != nil {
			t.Fatalf("Failed reading input dir: %s", err)
		}

		var pathsToTarUp []string
		for _, f := range filesInInputDir {
			pathsToTarUp = append(pathsToTarUp, path.Join(inputDir, f.Name()))
		}

		testDir, err := ioutil.TempDir("", "outputFolder")
		defer os.RemoveAll(testDir)

		tarPath := path.Join(testDir, "tar.tar")
		untarPath := path.Join(testDir, "untarred-data")
		if err != nil {
			t.Fatalf("Making temporary directory: %v", err)
		}

		if err := NewTar().Archive(pathsToTarUp, tarPath); err != nil {
			t.Fatalf("Failed to create tar archive: %s", err)
		}

		if err := NewTar().Unarchive(tarPath, untarPath); err != nil {
			t.Fatalf("Failed to untar archive: %s", err)
		}

		if output, err := exec.Command("diff", "-rq", inputDir, untarPath).Output(); err != nil {
			t.Fatalf("Folders are different! \n%s", string(output))
		}
	})

	t.Run("Create tar stream and untar it to a new location using ArchiveToStream/UnarchiveFromStream", func(t *testing.T) {
		filesInInputDir, err := ioutil.ReadDir(inputDir)
		if err != nil {
			t.Fatalf("Failed reading input dir: %s", err)
		}

		var pathsToTarUp []string
		for _, f := range filesInInputDir {
			pathsToTarUp = append(pathsToTarUp, path.Join(inputDir, f.Name()))
		}

		testDir, err := ioutil.TempDir("", "outputFolderFromStream")
		if err != nil {
			t.Fatalf("Making temporary directory: %v", err)
		}
		defer os.RemoveAll(testDir)

		tar := bytes.NewBuffer([]byte(""))
		if err := NewTar().ArchiveToStream(tar, pathsToTarUp); err != nil {
			t.Fatalf("Failed to create tar archive: %s", err)
		}

		if err := NewTar().UnarchiveFromStream(tar, testDir); err != nil {
			t.Fatalf("Failed to untar archive: %s", err)
		}

		if output, err := exec.Command("diff", "-rq", inputDir, testDir).Output(); err != nil {
			t.Fatalf("Folders are different! \n%s", string(output))
		}
	})
}
