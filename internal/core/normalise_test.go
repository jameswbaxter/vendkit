// Ported from tests/test_units.py — normalisation recipe (DR-0004).

package core

import "testing"

func TestCRLFAndTrailingWSInvisible(t *testing.T) {
	a, _ := NormaliseHash([]byte("line one\nline two\n"))
	b, _ := NormaliseHash([]byte("line one \r\nline two\t\r\n\r\n\r\n"))
	if a != b {
		t.Errorf("CRLF/trailing-ws should be invisible: %s != %s", a, b)
	}
}

func TestRealEditDetected(t *testing.T) {
	a, _ := NormaliseHash([]byte("alpha\n"))
	b, _ := NormaliseHash([]byte("alpha!\n"))
	if a == b {
		t.Error("a real content edit must change the hash")
	}
}

func TestBinaryIsRaw(t *testing.T) {
	input := []byte{0xff, 0xfe, 0x00, 'b', 'i', 'n', 'a', 'r', 'y'}
	digest, raw := NormaliseHash(input)
	if !raw {
		t.Error("non-UTF-8 input must be hashed raw")
	}
	if got := HashAsRecorded(input, true); got != digest {
		t.Errorf("HashAsRecorded(raw=true) = %s, want %s", got, digest)
	}
}

func TestEmptyFileStable(t *testing.T) {
	a, _ := NormaliseHash([]byte(""))
	b, _ := NormaliseHash([]byte("\n\n\n"))
	if a != b {
		t.Errorf("empty and all-newline files must hash alike: %s != %s", a, b)
	}
}
