//go:build darwin

package auth

import (
	"os/exec"
	"strings"
)

// probeKeychain reports whether macOS has a default keychain this process can
// write to.
//
// go-keyring stores secrets by running `security add-generic-password`. When
// the process's HOME has no login keychain, the Security framework does not
// return an error: it shows a SecurityAgent dialog whose default button is
// "Reset To Defaults", which resets the user's default keychain, and the command
// then exits 154. `security default-keychain` asks the same question without any
// UI, so probing with it first routes the store straight to its file fallback.
//
// This reports existence, not unlockedness: a locked keychain still exists, and
// macOS prompting to unlock it is expected and non-destructive.
var probeKeychain = func() bool {
	out, err := exec.Command("/usr/bin/security", "default-keychain").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}
