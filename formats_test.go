package archiver

import (
	"bytes"
	"io"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestRewindReader(t *testing.T) {
	data := "the header\nthe body\n"

	r := newRewindReader(strings.NewReader(data))

	buf := make([]byte, 10) // enough for 'the header'

	// test rewinding reads
	for i := 0; i < 10; i++ {
		r.rewind()
		n, err := r.Read(buf)
		if err != nil {
			t.Fatalf("Read failed: %s", err)
		}
		if string(buf[:n]) != "the header" {
			t.Fatalf("iteration %d: expected 'the header' but got '%s' (n=%d)", i, string(buf[:n]), n)
		}
	}

	// get the reader from header reader and make sure we can read all of the data out
	r.rewind()
	finalReader := r.reader()
	buf = make([]byte, len(data))
	n, err := io.ReadFull(finalReader, buf)
	if err != nil {
		t.Fatalf("ReadFull failed: %s (n=%d)", err, n)
	}
	if string(buf) != data {
		t.Fatalf("expected '%s' but got '%s'", string(data), string(buf))
	}
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

func checkErr(t *testing.T, err error, msgFmt string, args ...interface{}) {
	t.Helper()
	if err == nil {
		return
	}
	args = append(args, err)
	t.Fatalf(msgFmt+": %s", args...)
}
