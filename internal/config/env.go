package config

import (
	"os"

	"github.com/joho/godotenv"
)

// envBindings maps environment variable names to layer field keys.
var envBindings = map[string]string{
	"CONFLUENCE_AUTH_SCHEME":           fieldAuthScheme,
	"CONFLUENCE_CREDENTIAL_URL":        fieldCredentialURL,
	"CONFLUENCE_SERVER":                fieldServer,
	"CONFLUENCE_FLAVOR":                fieldFlavor,
	"CONFLUENCE_USERNAME":              fieldAuthUsername,
	"CONFLUENCE_FORMAT":                fieldFormat,
	"CONFLUENCE_PERSONAL_ACCESS_TOKEN": fieldPAT,
	"CONFLUENCE_PASSWORD":              fieldPassword,
	"CONFLUENCE_API_TOKEN":             fieldAPIToken,
	"CONFLUENCE_CLI_READ_ONLY":         fieldReadOnly,
}

// layerFromVars converts a name->value map into a layer map. Empty values are
// skipped so they do not shadow lower-precedence layers.
func layerFromVars(vars map[string]string, serviceOnly ...bool) map[string]string {
	m := map[string]string{}
	for name, field := range envBindings {
		if v := vars[name]; v != "" {
			if len(serviceOnly) > 0 && serviceOnly[0] && !serviceField(field) {
				continue
			}
			m[field] = v
		}
	}
	// If an auth secret is present infer the scheme so the user need not also
	// set CONFLUENCE_FLAVOR/scheme explicitly.
	if _, ok := m[fieldPAT]; ok {
		m[fieldAuthScheme] = SchemePAT
	} else if _, ok := m[fieldPassword]; ok {
		m[fieldAuthScheme] = SchemeBasic
	} else if _, ok := m[fieldAPIToken]; ok {
		m[fieldAuthScheme] = SchemeBasic
	}
	if v := vars["CONFLUENCE_AUTH_SCHEME"]; v != "" {
		m[fieldAuthScheme] = v
	}
	return m
}

// envLayer reads configuration from the process environment.
func envLayer(serviceOnly ...bool) map[string]string {
	vars := map[string]string{}
	for name := range envBindings {
		if v, ok := os.LookupEnv(name); ok {
			vars[name] = v
		}
	}
	return layerFromVars(vars, serviceOnly...)
}

// dotenvLayer reads configuration from a .env file without mutating the
// process environment. A missing file yields an empty layer.
func dotenvLayer(path string, serviceOnly ...bool) (map[string]string, error) {
	if path == "" {
		return map[string]string{}, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	vars, err := godotenv.Read(path)
	if err != nil {
		return nil, err
	}
	return layerFromVars(vars, serviceOnly...), nil
}
