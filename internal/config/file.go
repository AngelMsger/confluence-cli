package config

import (
	"os"
	"path/filepath"

	"github.com/angelmsger/confluence-cli/pkg/constants"
	"gopkg.in/yaml.v3"
)

// authShape / defaultsShape / contextShape are the on-disk YAML building
// blocks. Timeout is a human-readable duration string ("30s").
type authShape struct {
	Scheme   string `yaml:"scheme,omitempty"`
	Username string `yaml:"username,omitempty"`
}

type defaultsShape struct {
	Format     string `yaml:"format,omitempty"`
	PageSize   int    `yaml:"page_size,omitempty"`
	Timeout    string `yaml:"timeout,omitempty"`
	MaxRetries int    `yaml:"max_retries,omitempty"`
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
// runtime defaults and the name of the current context.
type File struct {
	CurrentContext string
	Contexts       []NamedContext
	Defaults       Defaults
}

// Context returns the context with the given name.
func (f File) Context(name string) (NamedContext, bool) {
	for _, c := range f.Contexts {
		if c.Name == name {
			return c, true
		}
	}
	return NamedContext{}, false
}

// ContextNames returns every context name, in file order.
func (f File) ContextNames() []string {
	names := make([]string, len(f.Contexts))
	for i, c := range f.Contexts {
		names[i] = c.Name
	}
	return names
}

// DefaultConfigDir returns the per-user config directory (~/.confluence).
func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, constants.ConfigDirName), nil
}

// ConfigFilePath returns the YAML config file path inside dir.
func ConfigFilePath(dir string) string {
	return filepath.Join(dir, constants.ConfigFileName)
}

// ReadFile reads and parses the config file in dir. The bool return is false
// when the file does not exist. A legacy flat-format file is normalized into a
// single context named "default".
func ReadFile(dir string) (File, bool, error) {
	raw, err := os.ReadFile(ConfigFilePath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, false, nil
		}
		return File{}, false, err
	}
	var fs fileShape
	if err := yaml.Unmarshal(raw, &fs); err != nil {
		return File{}, false, err
	}
	f := File{
		CurrentContext: fs.CurrentContext,
		Defaults:       defaultsFromShape(fs.Defaults),
	}
	switch {
	case len(fs.Contexts) > 0:
		for _, cs := range fs.Contexts {
			f.Contexts = append(f.Contexts, NamedContext{
				Name:           cs.Name,
				BaseURL:        cs.Server,
				Flavor:         cs.Flavor,
				DetectedFlavor: cs.DetectedFlavor,
				Auth:           AuthConfig{Scheme: cs.Auth.Scheme, Username: cs.Auth.Username},
			})
		}
	case fs.Server != "":
		// Legacy flat format → one synthesized "default" context.
		f.Contexts = []NamedContext{{
			Name:           DefaultContextName,
			BaseURL:        fs.Server,
			Flavor:         fs.Flavor,
			DetectedFlavor: fs.DetectedFlavor,
			Auth:           AuthConfig{Scheme: fs.Auth.Scheme, Username: fs.Auth.Username},
		}}
		if f.CurrentContext == "" {
			f.CurrentContext = DefaultContextName
		}
	}
	return f, true, nil
}

// WriteFile persists a multi-context File to dir/config.yaml in the new
// format, creating dir with 0700 permissions. Secrets are never written here.
func WriteFile(dir string, f File) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	var fs fileShape
	fs.CurrentContext = f.CurrentContext
	for _, c := range f.Contexts {
		fs.Contexts = append(fs.Contexts, contextShape{
			Name:           c.Name,
			Server:         c.BaseURL,
			Flavor:         c.Flavor,
			DetectedFlavor: c.DetectedFlavor,
			Auth:           authShape{Scheme: c.Auth.Scheme, Username: c.Auth.Username},
		})
	}
	fs.Defaults.Format = f.Defaults.Format
	fs.Defaults.PageSize = f.Defaults.PageSize
	if f.Defaults.Timeout > 0 {
		fs.Defaults.Timeout = f.Defaults.Timeout.String()
	}
	fs.Defaults.MaxRetries = f.Defaults.MaxRetries

	out, err := yaml.Marshal(&fs)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFilePath(dir), out, 0o600)
}

// defaultsFromShape converts the on-disk defaults block into a Defaults value.
// Missing fields stay zero; the default layer fills them during Load.
func defaultsFromShape(ds defaultsShape) Defaults {
	return Defaults{
		Format:     ds.Format,
		PageSize:   ds.PageSize,
		Timeout:    durationOr(ds.Timeout, 0),
		MaxRetries: ds.MaxRetries,
	}
}

func put(m map[string]string, key, val string) {
	if val != "" {
		m[key] = val
	}
}
