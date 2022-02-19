package archiver

import (
	"bytes"
	"io"
	"math/rand"
	"testing"
	"time"
)

func checkErr(t *testing.T, err error, msgFmt string, args ...interface{}) {
	t.Helper()
	if err == nil {
		return
	}
	args = append(args, err)
	t.Fatalf(msgFmt+": %s", args...)
}

func TestCompression(t *testing.T) {
	seed := time.Now().UnixNano()
	t.Logf("seed: %d", seed)
	r := rand.New(rand.NewSource(seed))

	contents := make([]byte, 1024)
	r.Read(contents)

	compressed := new(bytes.Buffer)

	testOK := func(t *testing.T, comp Compression, testFilename string) {
		// compress into buffer
		compressed.Reset()
		wc, err := comp.OpenWriter(compressed)
		checkErr(t, err, "opening writer")
		_, err = wc.Write(contents)
		checkErr(t, err, "writing contents")
		checkErr(t, wc.Close(), "closing writer")

		// make sure Identify correctly chooses this compression method
		format, stream, err := Identify(testFilename, compressed)
		checkErr(t, err, "identifying")
		if format.Name() != comp.Name() {
			t.Fatalf("expected format %s but got %s", comp.Name(), format.Name())
		}

		// read the contents back out and compare
		decompReader, err := format.(Decompressor).OpenReader(stream)
		checkErr(t, err, "opening with decompressor '%s'", format.Name())
		data, err := io.ReadAll(decompReader)
		checkErr(t, err, "reading decompressed data")
		checkErr(t, decompReader.Close(), "closing decompressor")
		if !bytes.Equal(data, contents) {
			t.Fatalf("not equal to original")
		}
	}

	var cannotIdentifyFromStream = map[string]bool{Brotli{}.Name(): true}

	for _, f := range formats {
		// only test compressors
		comp, ok := f.(Compression)
		if !ok {
			continue
		}

		t.Run(f.Name()+"_with_extension", func(t *testing.T) {
			testOK(t, comp, "file"+f.Name())
		})
		if !cannotIdentifyFromStream[f.Name()] {
			t.Run(f.Name()+"_without_extension", func(t *testing.T) {
				testOK(t, comp, "")
			})
		}
	}
}
