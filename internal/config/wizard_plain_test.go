package config

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// failingReader yields data, then fails with err instead of io.EOF.
type failingReader struct {
	data []byte
	err  error
}

func (r *failingReader) Read(b []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(b, r.data)
	r.data = r.data[n:]
	return n, nil
}

// An answer cut short by a read failure is not an answer: "yes" followed by
// a broken stdin must not confirm "Save configuration anyway?".
func TestPlainDriverReadFailureIsNotAnAnswer(t *testing.T) {
	broken := errors.New("stdin broke")
	p := NewPlainDriver(&failingReader{data: []byte("yes"), err: broken}, &bytes.Buffer{})
	ok, err := p.AskConfirm("Save configuration anyway?", false)
	if ok || !errors.Is(err, broken) {
		t.Fatalf("AskConfirm = %v, %v; want false and the read error", ok, err)
	}
}

// The end of input still completes a partial line, as scripted setups rely on.
func TestPlainDriverEndOfInputCompletesTheLine(t *testing.T) {
	p := NewPlainDriver(&failingReader{data: []byte("alice"), err: io.EOF}, &bytes.Buffer{})
	got, err := p.AskText("Username", "", "", true)
	if got != "alice" || err != nil {
		t.Fatalf("AskText = %q, %v; want alice", got, err)
	}
}
