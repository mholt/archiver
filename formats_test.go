package archiver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"math/rand"
	"os"
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

func TestIdentifyDoesNotMatchContentFromTrimmedKnownHeaderHaving0Suffix(t *testing.T) {
	// Using the outcome of `n, err := io.ReadFull(stream, buf)` without minding n
	// may lead to a mis-characterization for cases with known header ending with 0x0
	// because the default byte value in a declared array is 0.
	// This test guards against those cases.
	tests := []struct {
		name   string
		header []byte
	}{
		{
			name:   "rar_v5.0",
			header: rarHeaderV5_0,
		},
		{
			name:   "rar_v1.5",
			header: rarHeaderV1_5,
		},
		{
			name:   "xz",
			header: xzHeader,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headerLen := len(tt.header)
			if headerLen == 0 || tt.header[headerLen-1] != 0 {
				t.Errorf("header expected to end with 0: header=%v", tt.header)
				return
			}
			headerTrimmed := tt.header[:headerLen-1]
			stream := bytes.NewReader(headerTrimmed)
			got, _, err := Identify("", stream)
			if got != nil {
				t.Errorf("no Format expected for trimmed know %s header: found Format= %v", tt.name, got.Name())
				return
			}
			if ErrNoMatch != err {
				t.Fatalf("ErrNoMatch expected for for trimmed know %s header: err :=%#v", tt.name, err)
				return
			}

		})
	}
}

func TestIdentifyCanAssessSmallOrNoContent(t *testing.T) {
	type args struct {
		stream io.ReadSeeker
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "should return nomatch for an empty stream",
			args: args{
				stream: bytes.NewReader([]byte{}),
			},
		},
		{
			name: "should return nomatch for a stream with content size less than known header",
			args: args{
				stream: bytes.NewReader([]byte{'a'}),
			},
		},
		{
			name: "should return nomatch for a stream with content size greater then known header size and not supported format",
			args: args{
				stream: bytes.NewReader([]byte(strings.Repeat("this is a txt content", 2))),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Identify("", tt.args.stream)
			if got != nil {
				t.Errorf("no Format expected for non archive and not compressed stream: found Format= %v", got.Name())
				return
			}
			if ErrNoMatch != err {
				t.Fatalf("ErrNoMatch expected for non archive and not compressed stream: err :=%#v", err)
				return
			}

		})
	}
}

func compress(
	t *testing.T, compName string, content []byte,
	openwriter func(w io.Writer) (io.WriteCloser, error),
) []byte {
	buf := bytes.NewBuffer(make([]byte, 0, 128))
	cwriter, err := openwriter(buf)
	if err != nil {
		t.Fatalf("fail to open compression writer: compression-name=%s, err=%#v", compName, err)
		return nil
	}
	_, err = cwriter.Write(content)
	if err != nil {
		cerr := cwriter.Close()
		t.Fatalf(
			"fail to write using compression writer: compression-name=%s, err=%#v, close-err=%#v",
			compName, err, cerr)
		return nil
	}
	err = cwriter.Close()
	if err != nil {
		t.Fatalf("fail to close compression writer: compression-name=%s, err=%#v", compName, err)
		return nil
	}
	return buf.Bytes()
}

func archive(t *testing.T, arch Archiver, fname string, fileInfo fs.FileInfo) []byte {
	files := []File{
		{FileInfo: fileInfo, NameInArchive: "tmp.txt",
			Open: func() (io.ReadCloser, error) {
				return os.Open(fname)
			}},
	}
	buf := bytes.NewBuffer(make([]byte, 0, 128))
	err := arch.Archive(context.TODO(), buf, files)
	if err != nil {
		t.Fatalf("fail to create archive: err=%#v", err)
		return nil
	}
	return buf.Bytes()

}

type writeNopCloser struct{ io.Writer }

func (wnc writeNopCloser) Close() error { return nil }

func newWriteNopCloser(w io.Writer) (io.WriteCloser, error) {
	return writeNopCloser{w}, nil
}

func newTmpTextFile(t *testing.T, content string) (string, fs.FileInfo) {

	tmpTxtFile, err := os.CreateTemp("", "TestIdentifyFindFormatByStreamContent-tmp-*.txt")
	if err != nil {
		t.Fatalf("fail to create tmp test file for archive tests: err=%v", err)
		return "", nil
	}
	fname := tmpTxtFile.Name()

	if _, err = tmpTxtFile.Write([]byte(content)); err != nil {
		tmpTxtFile.Close()
		os.Remove(fname)
		t.Fatalf("fail to write content to tmp-txt-file: err=%#v", err)
		return "", nil
	}
	if err = tmpTxtFile.Close(); err != nil {
		os.Remove(fname)
		t.Fatalf("fail to close tmp-txt-file: err=%#v", err)
		return "", nil
	}
	fi, err := os.Stat(fname)
	if err != nil {
		os.Remove(fname)
		t.Fatalf("fail to get tmp-txt-file stats: err=%v", err)
		return "", nil
	}

	return fname, fi
}

func TestIdentifyFindFormatByStreamContent(t *testing.T) {
	tmpTxtFileName, tmpTxtFileInfo := newTmpTextFile(t, "this is text")
	t.Cleanup(func() {
		os.Remove(tmpTxtFileName)
	})

	tests := []struct {
		name                  string
		content               []byte
		openCompressionWriter func(w io.Writer) (io.WriteCloser, error)
		compressorName        string
		wantFormatName        string
	}{
		//TODO add test case for brotli when Brotli.Match() by stream content is implemented
		{
			name:                  "should recognize bz2",
			openCompressionWriter: Bz2{}.OpenWriter,
			content:               []byte("this is text"),
			compressorName:        ".bz2",
			wantFormatName:        ".bz2",
		},
		{
			name:                  "should recognize gz",
			openCompressionWriter: Gz{}.OpenWriter,
			content:               []byte("this is text"),
			compressorName:        ".gz",
			wantFormatName:        ".gz",
		},
		{
			name:                  "should recognize lz4",
			openCompressionWriter: Lz4{}.OpenWriter,
			content:               []byte("this is text"),
			compressorName:        ".lz4",
			wantFormatName:        ".lz4",
		},
		{
			name:                  "should recognize lz",
			openCompressionWriter: Lzip{}.OpenWriter,
			content:               []byte("this is text"),
			compressorName:        ".lz",
			wantFormatName:        ".lz",
		},
		{
			name:                  "should recognize sz",
			openCompressionWriter: Sz{}.OpenWriter,
			content:               []byte("this is text"),
			compressorName:        ".sz",
			wantFormatName:        ".sz",
		},
		{
			name:                  "should recognize xz",
			openCompressionWriter: Xz{}.OpenWriter,
			content:               []byte("this is text"),
			compressorName:        ".xz",
			wantFormatName:        ".xz",
		},
		{
			name:                  "should recognize zst",
			openCompressionWriter: Zstd{}.OpenWriter,
			content:               []byte("this is text"),
			compressorName:        ".zst",
			wantFormatName:        ".zst",
		},
		{
			name:                  "should recognize tar",
			openCompressionWriter: newWriteNopCloser,
			content:               archive(t, Tar{}, tmpTxtFileName, tmpTxtFileInfo),
			compressorName:        "",
			wantFormatName:        ".tar",
		},
		{
			name:                  "should recognize tar.gz",
			openCompressionWriter: Gz{}.OpenWriter,
			content:               archive(t, Tar{}, tmpTxtFileName, tmpTxtFileInfo),
			compressorName:        ".gz",
			wantFormatName:        ".tar.gz",
		},
		{
			name:                  "should recognize zip",
			openCompressionWriter: newWriteNopCloser,
			content:               archive(t, Zip{}, tmpTxtFileName, tmpTxtFileInfo),
			compressorName:        "",
			wantFormatName:        ".zip",
		},
		{
			name:                  "should recognize rar by v5.0 header",
			openCompressionWriter: newWriteNopCloser,
			content:               rarHeaderV5_0[:],
			compressorName:        "",
			wantFormatName:        ".rar",
		},
		{
			name:                  "should recognize rar by v1.5 header",
			openCompressionWriter: newWriteNopCloser,
			content:               rarHeaderV1_5[:],
			compressorName:        "",
			wantFormatName:        ".rar",
		},
		{
			name:                  "should recognize zz",
			openCompressionWriter: Zlib{}.OpenWriter,
			content:               []byte("this is text"),
			compressorName:        ".zz",
			wantFormatName:        ".zz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := bytes.NewReader(compress(t, tt.compressorName, tt.content, tt.openCompressionWriter))
			got, _, err := Identify("", stream)
			if err != nil {
				t.Fatalf("should have found a corresponding Format: err :=%+v", err)
				return
			}
			if tt.wantFormatName != got.Name() {
				t.Errorf("unexpected format found: expected=%s actual:%s", tt.wantFormatName, got.Name())
				return
			}

		})
	}
}

func TestIdentifyAndOpenZip(t *testing.T) {
	f, err := os.Open("testdata/test.zip")
	checkErr(t, err, "opening zip")
	defer f.Close()

	format, reader, err := Identify("test.zip", f)
	checkErr(t, err, "identifying zip")
	if format.Name() != ".zip" {
		t.Fatalf("unexpected format found: expected=.zip actual:%s", format.Name())
	}

	err = format.(Extractor).Extract(context.Background(), reader, nil, func(ctx context.Context, f File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		_, err = io.ReadAll(rc)
		return err
	})
	checkErr(t, err, "extracting zip")
}

func TestIdentifyASCIIFileStartingWithX(t *testing.T) {
	// Create a temporary file starting with the letter 'x'
	tmpFile, err := os.CreateTemp("", "TestIdentifyASCIIFileStartingWithX-tmp-*.txt")
	if err != nil {
		t.Fatalf("fail to create tmp test file for archive tests: err=%v", err)
	}

	_, err = tmpFile.Write([]byte("xThis is a test file"))
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Open the file and use the Identify function
	file, err := os.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open temp file: %v", err)
	}
	defer file.Close()

	_, _, err = Identify(tmpFile.Name(), file)
	if !errors.Is(err, ErrNoMatch) {
		t.Fatalf("Identify failed: %v", err)
	}

}
