package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angelmsger/agent-cli-kit/credstore"
	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/zalando/go-keyring"
)

type loginContractBackend struct {
	secret         string
	setErr, getErr error
}

func (b *loginContractBackend) Get(string, string) (string, error) { return b.secret, b.getErr }
func (b *loginContractBackend) Set(_, _, secret string) error {
	if b.setErr != nil {
		return b.setErr
	}
	b.secret = secret
	return nil
}
func (b *loginContractBackend) Delete(string, string) error { b.secret = ""; return nil }

// Exercise the real store and login boundary together; no host keychain or
// service is contacted. The error payload is what an agent actually receives.
func TestLoginCredentialStoreContract(t *testing.T) {
	for _, scenario := range []string{"success", "safe-fallback", "inaccessible", "partial-save"} {
		t.Run(scenario, func(t *testing.T) {
			s, cfg, cred := loginStateForTest(t)
			sentinel := errors.New("private-store-diagnostic")
			backend := &loginContractBackend{}
			if scenario == "safe-fallback" {
				backend.setErr = sentinel
				backend.getErr = keyring.ErrNotFound
			}
			if scenario == "inaccessible" {
				backend.setErr = sentinel
				backend.getErr = sentinel
			}
			s.store = credstore.NewStoreWithBackend(s.cfgDir, "login-contract-fixture", backend)
			if scenario == "partial-save" {
				if err := os.WriteFile(filepath.Join(s.cfgDir, credstore.CredentialsFileName), []byte("private-invalid-json"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			services := s.loginServices()
			services.Verify = func(config.Config, auth.Credential) error { return nil }
			storedIn, err := completeLogin(s, cfg, cred, services)
			if scenario == "success" || scenario == "safe-fallback" {
				want := credstore.BackendKeychain
				if scenario == "safe-fallback" {
					want = credstore.BackendFile
				}
				if err != nil || storedIn != want {
					t.Fatalf("login did not complete with expected backend: %v", err)
				}
				got, loadErr := s.store.Load(auth.AccountKey(cfg.BaseURL, cred.Scheme))
				if loadErr != nil || got != cred.Secret {
					t.Fatal("successful login did not persist the verified credential")
				}
				file, _, readErr := config.ReadFile(s.cfgDir)
				context, ok := file.Context("default")
				if readErr != nil || !ok || context.Auth.Username != cred.Username {
					t.Fatal("successful login did not persist matching identity")
				}
				return
			}
			if err == nil {
				t.Fatal("incomplete storage reported success")
			}
			if _, statErr := os.Stat(config.ConfigFilePath(s.cfgDir)); !os.IsNotExist(statErr) {
				t.Fatal("failed storage changed login identity")
			}
			ce := cerrors.AsCLIError(err)
			if ce.Code != "CREDENTIAL_SAVE_FAILED" || ce.Retryable || ce.Recovery == nil || ce.Recovery.Scope != "host" {
				t.Fatalf("missing safe storage recovery: %+v", ce)
			}
			raw, marshalErr := json.Marshal(ce.Payload())
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			for _, secret := range []string{cred.Secret, sentinel.Error(), "private-invalid-json"} {
				if strings.Contains(string(raw), secret) {
					t.Fatal("credential error exposed private data")
				}
			}
			var wire struct {
				Error struct {
					Details map[string]any `json:"details"`
				} `json:"error"`
			}
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatal(err)
			}
			if scenario == "partial-save" {
				var partial *credstore.PartialSaveError
				if !errors.As(err, &partial) || backend.secret != cred.Secret {
					t.Fatal("fixture did not reach real partial persistence")
				}
				if wire.Error.Details["credential_stored"] != true || wire.Error.Details["stored_in"] != "keychain" {
					t.Fatalf("lost persisted outcome: %s", raw)
				}
				if strings.Contains(ce.Message, "could not be stored") || strings.Contains(ce.Hint, "was not stored") {
					t.Fatal("reported a persisted credential as absent")
				}
			} else {
				if !errors.Is(err, sentinel) || backend.secret != "" || wire.Error.Details["credential_stored"] == true {
					t.Fatal("incorrect complete-failure outcome")
				}
			}
		})
	}
}
