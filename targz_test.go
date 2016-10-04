package archiver

import "testing"

func TestTarAndUntar(t *testing.T) {
	symmetricTest(t, ".tar", Tar, Untar)
}

func TestTarGzAndUntarGz(t *testing.T) {
	symmetricTest(t, ".tar.gz", TarGz, UntarGz)
}
