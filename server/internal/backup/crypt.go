package backup

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/scrypt"
)

// An encrypted backup is the plain archive sealed in fixed-size AES-256-GCM
// chunks under a key derived from the passphrase with scrypt:
//
//	magic | salt | chunk... | final chunk
//
// Each chunk's nonce is its index plus a final-chunk flag, so chunks cannot be
// reordered, dropped, or the archive cut short at a chunk boundary without
// failing authentication. The final chunk is always shorter than a full one
// (possibly empty), which is how the reader recognizes it.
var encMagic = []byte("LOGDECK-BACKUP-ENC1\n")

const (
	saltSize  = 16
	chunkSize = 64 << 10
	// scrypt cost: about 100 ms and 32 MB on a small VPS, once per backup.
	scryptN = 1 << 15
	scryptR = 8
	scryptP = 1
)

var ErrBadPassphrase = errors.New("the passphrase is wrong or the backup is corrupted")

func deriveKey(passphrase string, salt []byte) (cipher.AEAD, error) {
	key, err := scrypt.Key([]byte(passphrase), salt, scryptN, scryptR, scryptP, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func chunkNonce(index uint64, final bool) []byte {
	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce[3:11], index)
	if final {
		nonce[11] = 1
	}
	return nonce
}

// encryptWriter seals everything written to it; Close writes the final chunk.
type encryptWriter struct {
	w     io.Writer
	aead  cipher.AEAD
	buf   []byte
	out   []byte
	index uint64
}

func newEncryptWriter(w io.Writer, passphrase string) (*encryptWriter, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	aead, err := deriveKey(passphrase, salt)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(append(append([]byte{}, encMagic...), salt...)); err != nil {
		return nil, err
	}
	return &encryptWriter{w: w, aead: aead, buf: make([]byte, 0, chunkSize)}, nil
}

func (e *encryptWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		n := copy(e.buf[len(e.buf):chunkSize], p)
		e.buf = e.buf[:len(e.buf)+n]
		p = p[n:]
		written += n
		// Seal a full chunk only once more data follows, so the last chunk
		// Close seals is always short.
		if len(e.buf) == chunkSize && len(p) > 0 {
			if err := e.seal(false); err != nil {
				return written, err
			}
		}
	}
	return written, nil
}

func (e *encryptWriter) seal(final bool) error {
	e.out = e.aead.Seal(e.out[:0], chunkNonce(e.index, final), e.buf, nil)
	e.index++
	e.buf = e.buf[:0]
	_, err := e.w.Write(e.out)
	return err
}

func (e *encryptWriter) Close() error {
	if len(e.buf) == chunkSize {
		if err := e.seal(false); err != nil {
			return err
		}
	}
	return e.seal(true)
}

// decryptReader opens the chunks an encryptWriter produced. It returns
// ErrBadPassphrase for any chunk that fails authentication, including a
// missing final chunk.
type decryptReader struct {
	r     io.Reader
	aead  cipher.AEAD
	in    []byte
	out   []byte
	index uint64
	done  bool
}

func newDecryptReader(r io.Reader, passphrase string) (*decryptReader, error) {
	header := make([]byte, len(encMagic)+saltSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("read encryption header: %w", err)
	}
	aead, err := deriveKey(passphrase, header[len(encMagic):])
	if err != nil {
		return nil, err
	}
	return &decryptReader{r: r, aead: aead, in: make([]byte, chunkSize+aead.Overhead())}, nil
}

func (d *decryptReader) Read(p []byte) (int, error) {
	for len(d.out) == 0 {
		if d.done {
			return 0, io.EOF
		}
		n, err := io.ReadFull(d.r, d.in)
		final := false
		switch {
		case err == io.EOF || err == io.ErrUnexpectedEOF:
			final = true
		case err != nil:
			return 0, err
		}
		plain, err := d.aead.Open(d.in[:0], chunkNonce(d.index, final), d.in[:n], nil)
		if err != nil {
			return 0, ErrBadPassphrase
		}
		d.index++
		d.out = plain
		d.done = final
	}
	n := copy(p, d.out)
	d.out = d.out[n:]
	return n, nil
}

// isEncrypted reports whether the buffered stream starts with the encrypted
// backup header, without consuming it.
func isEncrypted(br *bufio.Reader) bool {
	head, _ := br.Peek(len(encMagic))
	return bytes.Equal(head, encMagic)
}
