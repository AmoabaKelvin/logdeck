package backup

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"testing"
)

func seal(t *testing.T, plain []byte, passphrase string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := newEncryptWriter(&buf, passphrase)
	if err != nil {
		t.Fatal(err)
	}
	// Uneven writes cross chunk boundaries at odd offsets.
	for rest := plain; len(rest) > 0; {
		n := min(len(rest), 1000)
		if _, err := w.Write(rest[:n]); err != nil {
			t.Fatal(err)
		}
		rest = rest[n:]
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func open(sealed []byte, passphrase string) ([]byte, error) {
	r, err := newDecryptReader(bytes.NewReader(sealed), passphrase)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

func TestEncryptionRoundTripsAtChunkBoundaries(t *testing.T) {
	for _, size := range []int{0, 1, chunkSize - 1, chunkSize, chunkSize + 1, 3 * chunkSize} {
		plain := make([]byte, size)
		_, _ = rand.Read(plain)
		got, err := open(seal(t, plain, "passphrase"), "passphrase")
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("size %d: plaintext changed", size)
		}
	}
}

func TestEncryptionRejectsWrongPassphraseTamperingAndTruncation(t *testing.T) {
	plain := make([]byte, 2*chunkSize)
	sealed := seal(t, plain, "passphrase")
	header := len(encMagic) + saltSize
	fullChunk := chunkSize + 16

	tampered := bytes.Clone(sealed)
	tampered[header+10] ^= 1

	cases := map[string]struct {
		data       []byte
		passphrase string
	}{
		"wrong passphrase":         {sealed, "other passphrase"},
		"flipped byte":             {tampered, "passphrase"},
		"cut at a chunk boundary":  {sealed[:header+fullChunk], "passphrase"},
		"cut inside a chunk":       {sealed[:header+fullChunk+100], "passphrase"},
		"final chunk dropped only": {sealed[:len(sealed)-16], "passphrase"},
	}
	for name, tc := range cases {
		if _, err := open(tc.data, tc.passphrase); !errors.Is(err, ErrBadPassphrase) {
			t.Errorf("%s: err = %v, want ErrBadPassphrase", name, err)
		}
	}
}
