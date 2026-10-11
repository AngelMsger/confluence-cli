package auth

import (
	"testing"

	"github.com/zalando/go-keyring"
)

// recordingKeyring records whether the store reached it at all.
type recordingKeyring struct {
	touched bool
	items   map[string]string
}

func (r *recordingKeyring) Get(_, account string) (string, error) {
	r.touched = true
	v, ok := r.items[account]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return v, nil
}

func (r *recordingKeyring) Set(_, account, secret string) error {
	r.touched = true
	r.items[account] = secret
	return nil
}

func (r *recordingKeyring) Delete(_, account string) error {
	r.touched = true
	delete(r.items, account)
	return nil
}

// When no default keychain exists, the store must use its file fallback without
// touching the OS keychain: on macOS that write raises the "Reset To Defaults"
// system dialog.
func TestStoreNeverTouchesAnUnavailableKeychain(t *testing.T) {
	kr := &recordingKeyring{items: map[string]string{}}
	s := newStoreWithKeyring(t.TempDir(), kr)
	s.probe = func() bool { return false }

	backend, err := s.Save("host:basic", "hunter2")
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if backend != BackendFile {
		t.Fatalf("Save() backend = %q, want %q", backend, BackendFile)
	}
	if got, err := s.Load("host:basic"); err != nil || got != "hunter2" {
		t.Fatalf("Load() = %q, %v; want hunter2, nil", got, err)
	}
	if err := s.Delete("host:basic"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.Load("host:basic"); err != ErrSecretNotFound {
		t.Fatalf("Load() after Delete error = %v, want ErrSecretNotFound", err)
	}
	if kr.touched {
		t.Fatal("the store reached the OS keychain although none is available")
	}
}

// The probe runs at most once per store, and NewStore wires the platform probe.
func TestStoreProbesKeychainOncePerStore(t *testing.T) {
	calls := 0
	prev := probeKeychain
	probeKeychain = func() bool { calls++; return false }
	t.Cleanup(func() { probeKeychain = prev })

	s := NewStore(t.TempDir())
	for i := 0; i < 3; i++ {
		if _, err := s.Save("host:basic", "x"); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}
	if calls != 1 {
		t.Fatalf("probe ran %d times, want 1", calls)
	}
}
