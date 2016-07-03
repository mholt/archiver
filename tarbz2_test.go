package archiver

import "testing"

func TestTarBz2AndUntarBz2(t *testing.T) {
	symmetricTest(t, ".tar.bz2", TarBz2, UntarBz2)
}
