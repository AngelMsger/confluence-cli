package config

// StoredContext resolves only persisted context data and built-in defaults.
// Credential reuse must not inherit another context's environment or .env identity.
func StoredContext(context NamedContext, defaults Defaults) Config {
	values := defaultLayer()
	sources := make(map[string]string, len(values))
	for key := range values {
		sources[key] = "default"
	}
	for key, value := range buildFileLayer(File{Contexts: []NamedContext{context}, Defaults: defaults}, context.Name) {
		values[key] = value
		sources[key] = "file"
	}
	resolveAuthDefaults(values, sources)
	return configFromMap(values)
}

// Equivalent flag spellings must not orphan a credential kept under a legacy key.
func preserveCredentialLookup(cfg Config, file File, context string) Config {
	stored, ok := file.Context(context)
	if !ok || stored.BaseURL == cfg.BaseURL {
		return cfg
	}
	left, e1 := NormalizeServiceURL(stored.BaseURL)
	right, e2 := NormalizeServiceURL(cfg.BaseURL)
	if e1 == nil && e2 == nil && left == right {
		cfg.CredentialBaseURL = stored.BaseURL
	}
	return cfg
}
