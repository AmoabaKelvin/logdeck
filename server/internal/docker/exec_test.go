package docker

import (
	"bytes"
	"testing"
)

func TestCapWriterKeepsHeadAndAcceptsTheRest(t *testing.T) {
	var w capWriter
	chunk := bytes.Repeat([]byte("x"), maxExecOutput-10)
	for range 3 {
		if n, err := w.Write(chunk); n != len(chunk) || err != nil {
			t.Fatalf("Write = %d, %v; want every byte accepted", n, err)
		}
	}
	if w.buf.Len() != maxExecOutput || !w.truncated {
		t.Fatalf("kept %d bytes, truncated=%v; want %d and true", w.buf.Len(), w.truncated, maxExecOutput)
	}
}
