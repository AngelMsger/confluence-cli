package auth

import (
	"github.com/angelmsger/agent-cli-kit/credstore"
	"github.com/angelmsger/confluence-cli/pkg/constants"
)

// Store persists secrets in the OS keychain, falling back to a protected file
// under the config directory when no keychain is available. It is
// agent-cli-kit's credstore under confluence-cli's keychain service name.
type Store = credstore.Store

// StoreAccessError means the credential store could not be inspected, as
// opposed to holding no secret (ErrSecretNotFound).
type StoreAccessError = credstore.StoreAccessError

// ErrSecretNotFound is returned by a Store when no secret exists for an account.
var ErrSecretNotFound = credstore.ErrSecretNotFound

// Backend names reported by Save.
const (
	BackendKeychain = credstore.BackendKeychain
	BackendFile     = credstore.BackendFile
)

// NewStore returns a Store whose file fallback lives in dir.
func NewStore(dir string) *Store {
	return credstore.NewStore(dir, constants.KeychainService)
}
