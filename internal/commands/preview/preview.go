// Package preview implements the "preview" command: compute entries since
// the last release and print the resulting notes to stdout, without
// touching the backend.
package preview

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/brpaz/draftsman/internal/commands/shared"
	"github.com/brpaz/draftsman/internal/config"
	"github.com/brpaz/draftsman/internal/engine"
)

const (
	name  = "preview"
	usage = "Print the computed release notes to stdout, without publishing anything"
)

// New returns the preview subcommand.
func New() *cli.Command {
	return &cli.Command{
		Name:  name,
		Usage: usage,
		Flags: []cli.Flag{
			shared.ConfigFlag(),
			// Backend/token/repo are optional here: without all three, PR
			// enrichment (see ADR-0001) is skipped, but commit-based
			// generation still works.
			shared.BackendFlag(false),
			shared.TokenFlag(false),
			shared.RepoFlag(false),
			shared.BaseURLFlag(),
			shared.PackageFlag(),
		},
		Action: run,
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	cfg, err := config.Load(cmd.String("config"), cmd.IsSet("config"))
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	b, err := shared.ResolveBackend(cmd, false)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	plan, err := engine.Compute(ctx, ".", cfg, b)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	_, _ = fmt.Fprint(cmd.Writer, plan.Rendered)
	return nil
}
