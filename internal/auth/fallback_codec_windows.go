//go:build windows

package auth

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsCredentialFilePrefix = "dpapi:v1:"

func encodeCredentialFile(plain []byte) ([]byte, error) {
	protected, err := protectWithDPAPI(plain)
	if err != nil {
		return nil, fmt.Errorf("protect credential fallback with DPAPI: %w", err)
	}
	return []byte(windowsCredentialFilePrefix + base64.StdEncoding.EncodeToString(protected)), nil
}

func decodeCredentialFile(raw []byte) ([]byte, error) {
	text := string(raw)
	if !strings.HasPrefix(text, windowsCredentialFilePrefix) {
		// Read legacy plaintext JSON so the next write can migrate it to DPAPI.
		return append([]byte(nil), raw...), nil
	}
	protected, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(text, windowsCredentialFilePrefix))
	if err != nil {
		return nil, fmt.Errorf("decode DPAPI credential fallback: %w", err)
	}
	plain, err := unprotectWithDPAPI(protected)
	if err != nil {
		return nil, fmt.Errorf("unprotect credential fallback with DPAPI: %w", err)
	}
	return plain, nil
}

func protectWithDPAPI(plain []byte) ([]byte, error) {
	in := dataBlob(plain)
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) //nolint:errcheck
	return copyDataBlob(out), nil
}

func unprotectWithDPAPI(protected []byte) ([]byte, error) {
	in := dataBlob(protected)
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) //nolint:errcheck
	return copyDataBlob(out), nil
}

func dataBlob(data []byte) windows.DataBlob {
	if len(data) == 0 {
		return windows.DataBlob{}
	}
	return windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
}

func copyDataBlob(blob windows.DataBlob) []byte {
	if blob.Size == 0 || blob.Data == nil {
		return nil
	}
	return append([]byte(nil), unsafe.Slice(blob.Data, int(blob.Size))...)
}
