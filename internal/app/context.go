package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
	"github.com/angelmsger/confluence-cli/internal/output"
	"github.com/angelmsger/confluence-cli/pkg/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// globalFlags holds the persistent flags shared by every command.
type globalFlags struct {
	baseURL    string
	flavor     string
	format     string
	fields     string
	timeout    string
	configPath string
	useContext string
	verbose    bool
	// pretty opts a human user into TUI prompts (in `config init`) and
	// ANSI-colored JSON (in every command that emits JSON). Off by default
	// so Agent / scripted / pipe usage stays byte-identical.
	pretty bool
	// allowWrites overrides read-only mode for the current invocation. The
	// posture itself is set via config (defaults.read_only) or env
	// (CONFLUENCE_CLI_READ_ONLY); this flag is the per-call escape hatch.
	allowWrites bool
}

// appState is the shared runtime context, built once in the root command's
// PersistentPreRunE and captured by every subcommand handler.
type appState struct {
	gflags   globalFlags
	resolved *config.Resolved
	store    *auth.Store
	cfgDir   string
}

// load resolves configuration from all sources using the current global flags.
func (s *appState) load() error {
	cfgDir := s.gflags.configPath
	if cfgDir == "" {
		d, err := config.ResolveConfigDir()
		if err != nil {
			return cerrors.Wrap(err, cerrors.CategoryConfig, "NO_HOME",
				"could not determine the home directory")
		}
		cfgDir = d
	}
	resolved, err := config.Load(config.LoadOptions{
		ConfigDir: cfgDir,
		Context:   s.gflags.useContext,
		Flags: config.FlagValues{
			BaseURL: s.gflags.baseURL,
			Flavor:  s.gflags.flavor,
			Format:  s.gflags.format,
			Timeout: s.gflags.timeout,
		},
	})
	if err != nil {
		// Pass structured CLI errors (e.g. UNKNOWN_CONTEXT) through untouched —
		// blanket-wrapping them buries the real reason in a generic CONFIG_LOAD
		// "failed to load configuration" message that strips the original hint
		// (e.g. "Available contexts: …"). Only opaque errors from file/dotenv
		// reads need the wrapper.
		var ce *cerrors.CLIError
		if errors.As(err, &ce) {
			return ce
		}
		return cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_LOAD",
			"failed to load configuration")
	}
	s.resolved = resolved
	s.cfgDir = cfgDir
	s.store = auth.NewStore(cfgDir)
	return nil
}

// cfg returns the resolved config.
func (s *appState) cfg() config.Config { return s.resolved.Config }

// newClient resolves credentials and builds an authenticated API client.
func (s *appState) newClient(ctx context.Context) (apiclient.Client, error) {
	cfg := s.cfg()
	cred, err := auth.Resolve(cfg, s.resolved.Secrets, s.store)
	if err != nil {
		return nil, err
	}
	client, _, err := apiclient.Build(ctx, apiclient.BuildParams{
		BaseURL:       cfg.BaseURL,
		Flavor:        cfg.Flavor,
		AuthDecorator: cred.Decorator(),
		Timeout:       cfg.Defaults.Timeout,
		MaxRetries:    cfg.Defaults.MaxRetries,
		PageSize:      cfg.Defaults.PageSize,
	})
	if err != nil {
		return nil, err
	}
	if s.readOnly() {
		client = apiclient.NewReadOnly(client)
	}
	return client, nil
}

// readOnly reports whether the effective posture for this invocation is
// read-only. The posture is set via config (defaults.read_only) or the env
// var CONFLUENCE_CLI_READ_ONLY; --allow-writes flips it back to read-write
// for the current call.
func (s *appState) readOnly() bool {
	return s.cfg().Defaults.ReadOnly && !s.gflags.allowWrites
}

// emit writes a successful result to stdout in the configured format.
func (s *appState) emit(v any) error {
	return output.Emit(v, output.Options{
		Format: s.cfg().Defaults.Format,
		Fields: s.fieldList(),
		Writer: os.Stdout,
		Pretty: s.gflags.pretty,
	})
}

// emitList writes a paginated list result to stdout as a {items, next,
// has_more} envelope in the configured format.
func (s *appState) emitList(items any, info pageInfo) error {
	return output.EmitList(items, info.Next, info.HasMore, output.Options{
		Format: s.cfg().Defaults.Format,
		Fields: s.fieldList(),
		Writer: os.Stdout,
		Pretty: s.gflags.pretty,
	})
}

// fieldList splits the --fields flag into dot paths.
func (s *appState) fieldList() []string {
	if s.gflags.fields == "" {
		return nil
	}
	parts := strings.Split(s.gflags.fields, ",")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// timeout returns the resolved request timeout.
func (s *appState) timeout() time.Duration { return s.cfg().Defaults.Timeout }
