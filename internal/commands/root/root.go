package root

import (
	"github.com/urfave/cli/v3"
)

const (
	Name  = "draftsman"
	usage = "CLI tool that generates release notes from Conventional Commits, maintaining a continuously-updated draft release across GitHub, GitLab, Gitea, and Forgejo"
)

// options holds the configuration for the root command.
type options struct {
	version  string
	commands []*cli.Command
}

// Option is a functional option for configuring the root command.
type Option func(*options)

// WithVersion sets the version string shown by --version.
func WithVersion(v string) Option {
	return func(o *options) { o.version = v }
}

// WithCommand appends a single sub-command to the root command.
// Call it multiple times to register multiple sub-commands.
func WithCommand(cmd *cli.Command) Option {
	return func(o *options) { o.commands = append(o.commands, cmd) }
}

// New returns the root *cli.Command with the supplied options applied.
func New(opts ...Option) *cli.Command {
	o := &options{
		version: "0.0.0-dev",
	}
	for _, opt := range opts {
		opt(o)
	}

	return &cli.Command{
		Name:                  Name,
		Version:               o.version,
		Usage:                 usage,
		EnableShellCompletion: true,
		Commands:              o.commands,
	}
}
