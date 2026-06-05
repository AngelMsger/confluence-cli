package apiclient

import (
	"context"
	"time"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/angelmsger/confluence-cli/internal/transport"
)

// BuildParams configures Build.
type BuildParams struct {
	BaseURL string
	// Flavor is "cloud", "datacenter" or "auto".
	Flavor string
	// AuthDecorator authenticates every request. Required.
	AuthDecorator transport.Decorator
	Timeout       time.Duration
	MaxRetries    int
	PageSize      int
}

// Build assembles a ready-to-use Client: it constructs the HTTP transport,
// resolves the flavor (probing the server when Flavor is "auto") and returns
// the client together with the resolved flavor.
func Build(ctx context.Context, p BuildParams) (Client, Flavor, error) {
	if p.BaseURL == "" {
		return nil, FlavorAuto, cerrors.New(cerrors.CategoryConfig, "NO_BASE_URL",
			"no Confluence server URL configured").
			WithNextSteps("confluence-cli config init", "Set CONFLUENCE_SERVER or pass --base-url.")
	}
	base := NormalizeBaseURL(p.BaseURL)

	tc := transport.New(transport.Options{
		Timeout:    p.Timeout,
		MaxRetries: p.MaxRetries,
		Decorators: []transport.Decorator{p.AuthDecorator},
	})

	flavor := Flavor(p.Flavor)
	switch flavor {
	case FlavorCloud, FlavorDataCenter:
		// explicit; trust it
	default:
		detected, err := Detect(ctx, tc, base)
		if err != nil {
			return nil, FlavorAuto, err
		}
		flavor = detected
	}

	client := New(Config{
		Flavor:    flavor,
		BaseURL:   base,
		PageSize:  p.PageSize,
		Transport: tc,
	})
	return client, flavor, nil
}
