// Package draft implements the "draft" command: compute entries since the
// last release and upsert the draft release(s) on the backend.
package draft

import (
	"context"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/commands/shared"
	"github.com/brpaz/draftsman/internal/config"
	"github.com/brpaz/draftsman/internal/engine"
	"github.com/brpaz/draftsman/internal/version"
)

const (
	name  = "draft"
	usage = "Upsert the draft release(s) with entries computed since the last release"
)

// New returns the draft subcommand.
func New() *cli.Command {
	return &cli.Command{
		Name:  name,
		Usage: usage,
		Flags: []cli.Flag{
			shared.ConfigFlag(),
			shared.BackendFlag(true),
			shared.TokenFlag(true),
			shared.RepoFlag(true),
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

	b, err := shared.ResolveBackend(cmd, true)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	plan, err := engine.Compute(ctx, ".", cfg, b)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	format, err := version.ParseFormat(cfg.TagFormat)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	if cfg.Mode == config.ModeMulti {
		return runMulti(ctx, cfg, b, plan, format, cmd.String("package"), cmd.Writer)
	}

	if plan.SuggestedVersion == "" {
		fmt.Fprintln(cmd.Writer, "nothing to release: no commits since the last release warrant one")
		return nil
	}

	tag := format.Render(plan.SuggestedVersion, "")

	if err := b.UpsertDraft(ctx, backend.UpsertDraftRequest{Tag: tag, Body: plan.Rendered}); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	fmt.Fprintf(cmd.Writer, "draft release upserted for %s\n", tag)
	return nil
}

// runMulti upserts one draft release per Package that has pending Entries
// (engine.Compute already omits Packages with none), each with its own
// {{package}}-scoped tag and its own isolated changelog body — never the
// combined multi-package document. --package, when set, scopes this to a
// single Package; omitted, every pending Package is processed (symmetric
// with draft's single-mode "process what's pending" behavior).
func runMulti(ctx context.Context, cfg *config.Config, b backend.Backend, plan *engine.Plan, format *version.Format, pkgFilter string, w io.Writer) error {
	upserted := 0
	for _, pp := range plan.Packages {
		if pkgFilter != "" && pp.Name != pkgFilter {
			continue
		}
		if pp.SuggestedVersion == "" {
			continue
		}

		body, err := engine.RenderPlan(cfg.Template, &engine.Plan{Packages: []engine.PackagePlan{pp}}, cfg.FooterEnabled())
		if err != nil {
			return fmt.Errorf("%s: rendering package %q: %w", name, pp.Name, err)
		}

		tag := format.Render(pp.SuggestedVersion, pp.Name)
		if err := b.UpsertDraft(ctx, backend.UpsertDraftRequest{Tag: tag, Body: body}); err != nil {
			return fmt.Errorf("%s: package %q: %w", name, pp.Name, err)
		}

		fmt.Fprintf(w, "draft release upserted for %s\n", tag)
		upserted++
	}

	if pkgFilter != "" && upserted == 0 {
		return fmt.Errorf("%s: package %q has nothing to release", name, pkgFilter)
	}
	if upserted == 0 {
		fmt.Fprintln(w, "nothing to release: no packages have commits since their last release that warrant one")
	}
	return nil
}
