package app

import (
	"context"

	draftcmd "github.com/brpaz/draftsman/internal/commands/draft"
	previewcmd "github.com/brpaz/draftsman/internal/commands/preview"
	publishcmd "github.com/brpaz/draftsman/internal/commands/publish"
	rootcmd "github.com/brpaz/draftsman/internal/commands/root"
)

// App is the composition root for the draftsman CLI.
type App struct {
	Info VersionInfo
}

// Option is a functional option for configuring an App.
type Option func(*App)

// WithVersionInfo sets the build-time version metadata.
func WithVersionInfo(info VersionInfo) Option {
	return func(a *App) { a.Info = info }
}

// New constructs an App with the provided options.
func New(opts ...Option) (*App, error) {
	appInstance := &App{
		Info: VersionInfo{
			Version:   "0.0.0-dev",
			Commit:    "n/a",
			BuildDate: "n/a",
		},
	}

	for _, opt := range opts {
		opt(appInstance)
	}

	return appInstance, nil
}

// Run builds the root command and executes it with the provided arguments.
func (app *App) Run(ctx context.Context, args []string) error {
	root := rootcmd.New(
		rootcmd.WithVersion(app.Info.String()),
		rootcmd.WithCommand(draftcmd.New()),
		rootcmd.WithCommand(previewcmd.New()),
		rootcmd.WithCommand(publishcmd.New()),
	)

	return root.Run(ctx, args)
}
