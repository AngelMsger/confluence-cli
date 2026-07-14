package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/angelmsger/confluence-cli/pkg/constants"
	"github.com/zalando/go-keyring"
)

// ErrSecretNotFound is returned by a Store when no secret exists for an account.
var ErrSecretNotFound = errors.New("secret not found")

// StoreAccessError means the credential store could not be inspected. It is
// intentionally distinct from ErrSecretNotFound: a caller must not tell the
// user to reconfigure credentials when a sandbox merely hid the keychain.
type StoreAccessError struct {
	Backend string
	Err     error
}

func (e *StoreAccessError) Error() string {
	return fmt.Sprintf("%s credential store is inaccessible: %v", e.Backend, e.Err)
}

func (e *StoreAccessError) Unwrap() error { return e.Err }

type keyringBackend interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
	Delete(service, account string) error
}

type systemKeyring struct{}

func (systemKeyring) Get(service, account string) (string, error) {
	return keyring.Get(service, account)
}
func (systemKeyring) Set(service, account, secret string) error {
	return keyring.Set(service, account, secret)
}
func (systemKeyring) Delete(service, account string) error {
	return keyring.Delete(service, account)
}

// Store persists secrets. It prefers the OS keychain and transparently falls
// back to a 0600 file under the config directory when the keychain is
// unavailable (headless Linux, CI, locked keychain).
type Store struct {
	dir     string // config directory for the file fallback
	keyring keyringBackend
}

// Backend names reported by Save.
const (
	BackendKeychain = "keychain"
	BackendFile     = "file"
)

// NewStore returns a Store whose file fallback lives in dir.
func NewStore(dir string) *Store { return newStoreWithKeyring(dir, systemKeyring{}) }

func newStoreWithKeyring(dir string, backend keyringBackend) *Store {
	return &Store{dir: dir, keyring: backend}
}

// Save stores secret for account and returns the backend that accepted it.
func (s *Store) Save(account, secret string) (string, error) {
	if err := s.keyring.Set(constants.KeychainService, account, secret); err == nil {
		return BackendKeychain, nil
	}
	if err := s.fileSave(account, secret); err != nil {
		return "", err
	}
	return BackendFile, nil
}

// Load retrieves the secret for account, trying the keychain then the file.
// It returns ErrSecretNotFound when neither holds a value.
func (s *Store) Load(account string) (string, error) {
	secret, keyringErr := s.keyring.Get(constants.KeychainService, account)
	if keyringErr == nil {
		return secret, nil
	}

	// A file fallback may still be usable when the platform keychain is locked
	// or unavailable, so try it before surfacing the keychain failure.
	secret, fileErr := s.fileLoad(account)
	if fileErr == nil {
		return secret, nil
	}

	keyringMissing := errors.Is(keyringErr, keyring.ErrNotFound)
	fileMissing := errors.Is(fileErr, ErrSecretNotFound)
	switch {
	case keyringMissing && fileMissing:
		return "", ErrSecretNotFound
	case !keyringMissing && fileMissing:
		return "", &StoreAccessError{Backend: BackendKeychain, Err: keyringErr}
	case keyringMissing:
		return "", &StoreAccessError{Backend: BackendFile, Err: fileErr}
	default:
		return "", &StoreAccessError{Backend: "keychain,file", Err: errors.Join(keyringErr, fileErr)}
	}
}

// Delete removes the secret for account from both backends. Missing entries
// are not an error.
func (s *Store) Delete(account string) error {
	_ = s.keyring.Delete(constants.KeychainService, account)
	return s.fileDelete(account)
}

func (s *Store) credentialsPath() string {
	return filepath.Join(s.dir, constants.CredentialsFileName)
}

func (s *Store) fileReadAll() (map[string]string, error) {
	raw, err := os.ReadFile(s.credentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	m := map[string]string{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (s *Store) fileWriteAll(m map[string]string) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.credentialsPath(), out, 0o600)
}

func (s *Store) fileSave(account, secret string) error {
	m, err := s.fileReadAll()
	if err != nil {
		return err
	}
	m[account] = secret
	return s.fileWriteAll(m)
}

func (s *Store) fileLoad(account string) (string, error) {
	m, err := s.fileReadAll()
	if err != nil {
		return "", err
	}
	secret, ok := m[account]
	if !ok {
		return "", ErrSecretNotFound
	}
	return secret, nil
}

func (s *Store) fileDelete(account string) error {
	m, err := s.fileReadAll()
	if err != nil {
		return err
	}
	if _, ok := m[account]; !ok {
		return nil
	}
	delete(m, account)
	return s.fileWriteAll(m)
}
