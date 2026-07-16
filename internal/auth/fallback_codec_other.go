//go:build !windows

package auth

func encodeCredentialFile(plain []byte) ([]byte, error) {
	return append([]byte(nil), plain...), nil
}

func decodeCredentialFile(raw []byte) ([]byte, error) {
	return append([]byte(nil), raw...), nil
}
