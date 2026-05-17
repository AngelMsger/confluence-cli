package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/angelmsger/confluence-cli/pkg/constants"
	"gopkg.in/yaml.v3"
)

// fileShape is the on-disk YAML representation of the config file. Timeout is a
// human-readable duration string ("30s") rather than raw nanoseconds.
type fileShape struct {
	Server         string `yaml:"server,omitempty"`
	Flavor         string `yaml:"flavor,omitempty"`
	DetectedFlavor string `yaml:"detected_flavor,omitempty"`
	Auth           struct {
		Scheme   string `yaml:"scheme,omitempty"`
		Username string `yaml:"username,omitempty"`
	} `yaml:"auth,omitempty"`
	Defaults struct {
		Format     string `yaml:"format,omitempty"`
		PageSize   int    `yaml:"page_size,omitempty"`
		Timeout    string `yaml:"timeout,omitempty"`
		MaxRetries int    `yaml:"max_retries,omitempty"`
	} `yaml:"defaults,omitempty"`
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

// readFileLayer loads the YAML config file at path into a layer map. A missing
// file yields an empty map and no error.
func readFileLayer(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	var fs fileShape
	if err := yaml.Unmarshal(raw, &fs); err != nil {
		return nil, err
	}
	m := map[string]string{}
	put(m, fieldServer, fs.Server)
	put(m, fieldFlavor, fs.Flavor)
	put(m, fieldDetectedFlavor, fs.DetectedFlavor)
	put(m, fieldAuthScheme, fs.Auth.Scheme)
	put(m, fieldAuthUsername, fs.Auth.Username)
	put(m, fieldFormat, fs.Defaults.Format)
	if fs.Defaults.PageSize > 0 {
		m[fieldPageSize] = strconv.Itoa(fs.Defaults.PageSize)
	}
	put(m, fieldTimeout, fs.Defaults.Timeout)
	if fs.Defaults.MaxRetries > 0 {
		m[fieldMaxRetries] = strconv.Itoa(fs.Defaults.MaxRetries)
	}
	return m, nil
}

// WriteConfigFile persists a Config to dir/config.yaml, creating dir with 0700
// permissions. Secrets are not part of Config and are never written here.
func WriteConfigFile(dir string, c Config) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	var fs fileShape
	fs.Server = c.BaseURL
	fs.Flavor = c.Flavor
	fs.DetectedFlavor = c.DetectedFlavor
	fs.Auth.Scheme = c.Auth.Scheme
	fs.Auth.Username = c.Auth.Username
	fs.Defaults.Format = c.Defaults.Format
	fs.Defaults.PageSize = c.Defaults.PageSize
	if c.Defaults.Timeout > 0 {
		fs.Defaults.Timeout = c.Defaults.Timeout.String()
	}
	fs.Defaults.MaxRetries = c.Defaults.MaxRetries

	out, err := yaml.Marshal(&fs)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFilePath(dir), out, 0o600)
}

func put(m map[string]string, key, val string) {
	if val != "" {
		m[key] = val
	}
}
