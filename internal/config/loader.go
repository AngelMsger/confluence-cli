package config

// FlagValues carries the global CLI flags that override configuration. Empty
// fields are ignored (not treated as overrides).
type FlagValues struct {
	BaseURL string
	Flavor  string
	Format  string
	Timeout string
}

func (f FlagValues) layer() map[string]string {
	m := map[string]string{}
	put(m, fieldServer, f.BaseURL)
	put(m, fieldFlavor, f.Flavor)
	put(m, fieldFormat, f.Format)
	put(m, fieldTimeout, f.Timeout)
	return m
}

// LoadOptions controls where configuration is read from. All fields are
// optional; sensible defaults are used when empty.
type LoadOptions struct {
	// ConfigDir overrides the directory containing config.yaml.
	ConfigDir string
	// DotenvPath overrides the .env file path. Empty means ".env".
	DotenvPath string
	// Flags carries global flag overrides (highest precedence).
	Flags FlagValues
}

type namedLayer struct {
	name string
	data map[string]string
}

// Load resolves configuration from all sources and returns the merged result
// with per-field provenance.
func Load(opt LoadOptions) (*Resolved, error) {
	dir := opt.ConfigDir
	if dir == "" {
		d, err := DefaultConfigDir()
		if err != nil {
			return nil, err
		}
		dir = d
	}
	fileLayer, err := readFileLayer(ConfigFilePath(dir))
	if err != nil {
		return nil, err
	}
	dotenvPath := opt.DotenvPath
	if dotenvPath == "" {
		dotenvPath = ".env"
	}
	dotLayer, err := dotenvLayer(dotenvPath)
	if err != nil {
		return nil, err
	}

	// Lowest precedence first.
	layers := []namedLayer{
		{"default", defaultLayer()},
		{"file", fileLayer},
		{"dotenv", dotLayer},
		{"env", envLayer()},
		{"flag", opt.Flags.layer()},
	}

	merged := map[string]string{}
	sources := map[string]string{}
	for _, l := range layers {
		for k, v := range l.data {
			merged[k] = v
			sources[k] = l.name
		}
	}

	return &Resolved{
		Config: configFromMap(merged),
		Secrets: Secrets{
			PAT:      merged[fieldPAT],
			Password: merged[fieldPassword],
			APIToken: merged[fieldAPIToken],
		},
		Sources: sources,
	}, nil
}

// ExplainField returns a human-readable provenance label for a field key,
// e.g. ExplainField(sources, "server") -> "env". Unknown fields report "default".
func ExplainField(sources map[string]string, field string) string {
	if s, ok := sources[field]; ok {
		return s
	}
	return "default"
}

// Field key accessors for callers outside this package (e.g. config show).
const (
	FieldServer     = fieldServer
	FieldFlavor     = fieldFlavor
	FieldAuthScheme = fieldAuthScheme
	FieldAuthUser   = fieldAuthUsername
	FieldFormat     = fieldFormat
	FieldTimeout    = fieldTimeout
	FieldPageSize   = fieldPageSize
	FieldMaxRetries = fieldMaxRetries
)
