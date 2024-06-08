package archiver

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path"
	"strings"
)

func init() {
	RegisterFormat(Tar{})
}

type Tar struct {
	// If true, preserve only numeric user and group id
	NumericUIDGID bool

	// If true, errors encountered during reading or writing
	// a file within an archive will be logged and the
	// operation will continue on remaining files.
	ContinueOnError bool
}

func (Tar) Name() string { return ".tar" }

func (t Tar) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), t.Name()) {
		mr.ByName = true
	}

	// match file header
	if stream != nil {
		r := tar.NewReader(stream)
		_, err := r.Next()
		mr.ByStream = err == nil
	}

	return mr, nil
}

func (t Tar) Archive(ctx context.Context, output io.Writer, files []File) error {
	tw := tar.NewWriter(output)
	defer tw.Close()

	for _, file := range files {
		if err := t.writeFileToArchive(ctx, tw, file); err != nil {
			if t.ContinueOnError && ctx.Err() == nil { // context errors should always abort
				log.Printf("[ERROR] %v", err)
				continue
			}
			return err
		}
	}

	return nil
}

func (t Tar) ArchiveAsync(ctx context.Context, output io.Writer, jobs <-chan ArchiveAsyncJob) error {
	tw := tar.NewWriter(output)
	defer tw.Close()

	for job := range jobs {
		job.Result <- t.writeFileToArchive(ctx, tw, job.File)
	}

	return nil
}

func (t Tar) writeFileToArchive(ctx context.Context, tw *tar.Writer, file File) error {
	if err := ctx.Err(); err != nil {
		return err // honor context cancellation
	}

	hdr, err := tar.FileInfoHeader(file, file.LinkTarget)
	if err != nil {
		return fmt.Errorf("file %s: creating header: %w", file.NameInArchive, err)
	}
	hdr.Name = file.NameInArchive // complete path, since FileInfoHeader() only has base name
	if hdr.Name == "" {
		hdr.Name = file.Name() // assume base name of file I guess
	}
	if t.NumericUIDGID {
		hdr.Uname = ""
		hdr.Gname = ""
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("file %s: writing header: %w", file.NameInArchive, err)
	}

	// only proceed to write a file body if there is actually a body
	// (for example, directories and links don't have a body)
	if hdr.Typeflag != tar.TypeReg {
		return nil
	}

	if err := openAndCopyFile(file, tw); err != nil {
		return fmt.Errorf("file %s: writing data: %w", file.NameInArchive, err)
	}

	return nil
}

func (t Tar) Insert(ctx context.Context, into io.ReadWriteSeeker, files []File) error {
	// Tar files may end with some, none, or a lot of zero-byte padding. The spec says
	// it should end with two 512-byte trailer records consisting solely of null/0
	// bytes: https://www.gnu.org/software/tar/manual/html_node/Standard.html. However,
	// in my experiments using the `tar` command, I've found that is not the case,
	// and Colin Percival (author of tarsnap) confirmed this:
	// - https://twitter.com/cperciva/status/1476774314623913987
	// - https://twitter.com/cperciva/status/1476776999758663680
	// So while this solution on Stack Overflow makes sense if you control the
	// writer: https://stackoverflow.com/a/18330903/1048862 - and I did get it
	// to work in that case -- it is not a general solution. Seems that the only
	// reliable thing to do is scan the entire archive to find the last file,
	// read its size, then use that to compute the end of content and thus the
	// true length of end-of-archive padding. This is slightly more complex than
	// just adding the size of the last file to the current stream/seek position,
	// because we have to align to 512-byte blocks precisely. I don't actually
	// fully know why this works, but in my testing on a few different files it
	// did work, whereas other solutions only worked on 1 specific file. *shrug*
	//
	// Another option is to scan the file for the last contiguous series of 0s,
	// without interpreting the tar format at all, and to find the nearest
	// blocksize-offset and start writing there. Problem is that you wouldn't
	// know if you just overwrote some of the last file if it ends with all 0s.
	// Sigh.
	var lastFileSize, lastStreamPos int64
	tr := tar.NewReader(into)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		lastStreamPos, err = into.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		lastFileSize = hdr.Size
	}

	// we can now compute the precise location to write the new file to (I think)
	const blockSize = 512 // (as of Go 1.17, this is also a hard-coded const in the archive/tar package)
	newOffset := lastStreamPos + lastFileSize
	newOffset += blockSize - (newOffset % blockSize) // shift to next-nearest block boundary
	_, err := into.Seek(newOffset, io.SeekStart)
	if err != nil {
		return err
	}

	tw := tar.NewWriter(into)
	defer tw.Close()

	for i, file := range files {
		if err := ctx.Err(); err != nil {
			return err // honor context cancellation
		}
		err = t.writeFileToArchive(ctx, tw, file)
		if err != nil {
			if t.ContinueOnError && ctx.Err() == nil {
				log.Printf("[ERROR] appending file %d into archive: %s: %v", i, file.Name(), err)
				continue
			}
			return fmt.Errorf("appending file %d into archive: %s: %w", i, file.Name(), err)
		}
	}

	return nil
}

func (t Tar) Extract(ctx context.Context, sourceArchive io.Reader, pathsInArchive []string, handleFile FileHandler) error {
	tr := tar.NewReader(sourceArchive)

	// important to initialize to non-nil, empty value due to how fileIsIncluded works
	skipDirs := skipList{}

	for {
		if err := ctx.Err(); err != nil {
			return err // honor context cancellation
		}

		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			if t.ContinueOnError && ctx.Err() == nil {
				log.Printf("[ERROR] Advancing to next file in tar archive: %v", err)
				continue
			}
			return err
		}
		if !fileIsIncluded(pathsInArchive, hdr.Name) {
			continue
		}
		if fileIsIncluded(skipDirs, hdr.Name) {
			continue
		}
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			// ignore the pax global header from git-generated tarballs
			continue
		}

		file := File{
			FileInfo:      hdr.FileInfo(),
			Header:        hdr,
			NameInArchive: hdr.Name,
			LinkTarget:    hdr.Linkname,
			Open:          func() (io.ReadCloser, error) { return io.NopCloser(tr), nil },
		}

		err = handleFile(ctx, file)
		if errors.Is(err, fs.SkipAll) {
			break
		} else if errors.Is(err, fs.SkipDir) {
			// if a directory, skip this path; if a file, skip the folder path
			dirPath := hdr.Name
			if hdr.Typeflag != tar.TypeDir {
				dirPath = path.Dir(hdr.Name) + "/"
			}
			skipDirs.add(dirPath)
		} else if err != nil {
			return fmt.Errorf("handling file: %s: %w", hdr.Name, err)
		}
	}

	return nil
}

// Interface guards
var (
	_ Archiver      = (*Tar)(nil)
	_ ArchiverAsync = (*Tar)(nil)
	_ Extractor     = (*Tar)(nil)
	_ Inserter      = (*Tar)(nil)
)
