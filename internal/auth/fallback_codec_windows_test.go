//go:build windows

package auth

import (
	"bytes"
	"strings"
	"testing"
)

func TestWindowsCredentialFileCodec(t *testing.T) {
	plain := []byte(`{"account":"fallback-secret"}`)
	encoded, err := encodeCredentialFile(plain)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(encoded), windowsCredentialFilePrefix) {
		t.Fatalf("encoded fallback does not have %q prefix", windowsCredentialFilePrefix)
	}
	if bytes.Contains(encoded, []byte("fallback-secret")) {
		t.Fatal("encoded fallback contains the plaintext secret")
	}
	decoded, err := decodeCredentialFile(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, plain) {
		t.Fatalf("decoded fallback = %q, want %q", decoded, plain)
	}
}

func TestWindowsCredentialFileCodecReadsLegacyPlaintext(t *testing.T) {
	legacy := []byte(`{"account":"legacy-secret"}`)
	decoded, err := decodeCredentialFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, legacy) {
		t.Fatalf("decoded legacy fallback = %q, want %q", decoded, legacy)
	}
}
