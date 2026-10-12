package config

import (
	"time"

	kit "github.com/angelmsger/agent-cli-kit/config"
)

// authShape / defaultsShape / contextShape are the on-disk YAML building
// blocks. Timeout is a human-readable duration string ("30s").
type authShape struct {
	CredentialURL string `yaml:"credential_url,omitempty"`
	Scheme        string `yaml:"scheme,omitempty"`
	Username      string `yaml:"username,omitempty"`
}

type defaultsShape struct {
	Format     string `yaml:"format,omitempty"`
	PageSize   int    `yaml:"page_size,omitempty"`
	Timeout    string `yaml:"timeout,omitempty"`
	MaxRetries int    `yaml:"max_retries,omitempty"`
	ReadOnly   bool   `yaml:"read_only,omitempty"`
}

type contextShape struct {
	Name           string    `yaml:"name"`
	Server         string    `yaml:"server,omitempty"`
	Flavor         string    `yaml:"flavor,omitempty"`
	DetectedFlavor string    `yaml:"detected_flavor,omitempty"`
	Auth           authShape `yaml:"auth,omitempty"`
}

// fileShape is the on-disk YAML representation of the config file. It carries
// both the legacy flat fields (server/flavor/auth at the top level) and the
// multi-context fields (current_context/contexts); only one form is populated
// in a given file.
type fileShape struct {
	// Legacy flat fields — still parsed for backward compatibility.
	Server         string    `yaml:"server,omitempty"`
	Flavor         string    `yaml:"flavor,omitempty"`
	DetectedFlavor string    `yaml:"detected_flavor,omitempty"`
	Auth           authShape `yaml:"auth,omitempty"`
	// Multi-context fields.
	CurrentContext string         `yaml:"current_context,omitempty"`
	Contexts       []contextShape `yaml:"contexts,omitempty"`
	Defaults       defaultsShape  `yaml:"defaults,omitempty"`
}

// File is the parsed config file: a set of named contexts plus the shared
// runtime defaults and the name of the current context. Context looks a name
// up case-insensitively and returns the stored spelling; callers that persist
// a reference to a context must use that spelling, not the lookup string.
type File = kit.File[NamedContext, Defaults]

// DefaultConfigDir returns the new-form per-user config directory
// (~/.angelmsger/confluence). Callers that want the smart "prefer new,
// fall back to legacy" resolution should use ResolveConfigDir instead;
// this function is the unconditional new path used by writes.
func DefaultConfigDir() (string, error) { return kit.DefaultDir() }

// LegacyConfigDir returns the pre-0.7 config directory (~/.confluence).
// It is consulted as a read-write fallback by ResolveConfigDir.
func LegacyConfigDir() (string, error) { return kit.LegacyDir() }

// ResolveConfigDir picks the config directory to use when --config was not
// supplied. It prefers the new ~/.angelmsger/confluence location, but falls
// back to the legacy ~/.confluence directory when the new path has no
// config.yaml yet and the legacy path does — so existing users keep working
// without an explicit migration step. When neither directory has a config
// file it returns the new path (so first-time setup writes there).
func ResolveConfigDir() (string, error) { return kit.ResolveDir() }

// ConfigFilePath returns the YAML config file path inside dir.
func ConfigFilePath(dir string) string { return kit.FilePath(dir) }

// UseContext makes the named context current and returns its stored name, or
// UNKNOWN_CONTEXT when there is no such context.
func UseContext(f *File, name string) (string, error) { return kit.UseContext(f, name) }

// DeleteContext removes the named context and returns it; the first remaining
// context becomes current when it was. It returns UNKNOWN_CONTEXT or
// LAST_CONTEXT instead when the name is unknown or the context is the only one.
func DeleteContext(f *File, name string) (NamedContext, error) { return kit.DeleteContext(f, name) }

// ReadFile reads and parses the config file in dir. The bool return is false
// when the file does not exist. A legacy flat-format file is normalized into a
// single context named "default".
func ReadFile(dir string) (File, bool, error) {
	var fs fileShape
	if ok, err := kit.ReadFile(dir, &fs); !ok || err != nil {
		return File{}, false, err
	}
	// Missing defaults stay zero; the default layer fills them during Load.
	timeout, _ := time.ParseDuration(fs.Defaults.Timeout)
	f := File{
		CurrentContext: fs.CurrentContext,
		Defaults: Defaults{
			Format:     fs.Defaults.Format,
			PageSize:   fs.Defaults.PageSize,
			Timeout:    timeout,
			MaxRetries: fs.Defaults.MaxRetries,
			ReadOnly:   fs.Defaults.ReadOnly,
		},
	}
	switch {
	case len(fs.Contexts) > 0:
		for _, cs := range fs.Contexts {
			f.Contexts = append(f.Contexts, contextFromShape(cs))
		}
	case fs.Server != "":
		// Legacy flat format → one synthesized "default" context.
		f.Contexts = []NamedContext{contextFromShape(contextShape{
			Name:           DefaultContextName,
			Server:         fs.Server,
			Flavor:         fs.Flavor,
			DetectedFlavor: fs.DetectedFlavor,
			Auth:           fs.Auth,
		})}
		if f.CurrentContext == "" {
			f.CurrentContext = DefaultContextName
		}
	}
	return f, true, nil
}

// WriteFile persists a multi-context File to dir/config.yaml in the new
// format, creating dir with 0700 permissions and replacing the file
// atomically. Secrets are never written here.
func WriteFile(dir string, f File) error {
	fs := fileShape{
		CurrentContext: f.CurrentContext,
		Defaults: defaultsShape{
			Format:     f.Defaults.Format,
			PageSize:   f.Defaults.PageSize,
			MaxRetries: f.Defaults.MaxRetries,
			ReadOnly:   f.Defaults.ReadOnly,
		},
	}
	if f.Defaults.Timeout > 0 {
		fs.Defaults.Timeout = f.Defaults.Timeout.String()
	}
	for _, c := range f.Contexts {
		fs.Contexts = append(fs.Contexts, contextShape{
			Name:           c.Name,
			Server:         c.BaseURL,
			Flavor:         c.Flavor,
			DetectedFlavor: c.DetectedFlavor,
			Auth:           authShape{Scheme: c.Auth.Scheme, Username: c.Auth.Username, CredentialURL: c.Auth.CredentialURL},
		})
	}
	return kit.WriteFile(dir, &fs)
}

func contextFromShape(cs contextShape) NamedContext {
	return NamedContext{
		Name:           cs.Name,
		BaseURL:        cs.Server,
		Flavor:         cs.Flavor,
		DetectedFlavor: cs.DetectedFlavor,
		Auth:           AuthConfig{Scheme: cs.Auth.Scheme, Username: cs.Auth.Username, CredentialURL: cs.Auth.CredentialURL},
	}
}
