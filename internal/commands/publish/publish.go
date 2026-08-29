// Package publish implements the "publish" command: promote a draft
// release to published, tagging it on the backend.
package publish

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
	name  = "publish"
	usage = "Promote the draft release to published and tag it"
)

// New returns the publish subcommand.
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
			&cli.StringFlag{
				Name:  "version",
				Usage: "override the auto-computed version instead of accepting the suggestion",
			},
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
		return runMulti(ctx, b, plan, format, cmd.String("package"), cmd.String("version"), cmd.Writer)
	}

	ver := cmd.String("version")
	if ver == "" {
		ver = plan.SuggestedVersion
	}
	if ver == "" {
		return fmt.Errorf("%s: no version to publish (nothing computed and --version not given)", name)
	}
	tag := format.Render(ver, "")

	if err := b.Publish(ctx, tag); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	fmt.Fprintf(cmd.Writer, "published %s\n", tag)
	return nil
}

// runMulti publishes one draft release per pending Package. --package scopes
// this to a single Package (required to use --version too — an override
// otherwise can't say which Package's tag it applies to; scoped this way, it
// also lets a Package with no currently-pending Entries still be published
// by explicit version, e.g. re-running publish for a draft made earlier).
// --package omitted publishes every Package with a pending draft in one
// invocation: publish's job is to finalize whatever draft already exists, so
// "publish everything that's ready" mirrors draft's own "process what's
// pending" default rather than forcing one invocation per Package.
func runMulti(ctx context.Context, b backend.Backend, plan *engine.Plan, format *version.Format, pkgFilter, verOverride string, w io.Writer) error {
	if verOverride != "" && pkgFilter == "" {
		return fmt.Errorf("%s: --version requires --package in multi mode (ambiguous which package it applies to)", name)
	}

	if pkgFilter != "" {
		ver := verOverride
		if ver == "" {
			ver = suggestedVersionFor(plan, pkgFilter)
		}
		if ver == "" {
			return fmt.Errorf("%s: package %q has nothing to publish", name, pkgFilter)
		}

		tag := format.Render(ver, pkgFilter)
		if err := b.Publish(ctx, tag); err != nil {
			return fmt.Errorf("%s: package %q: %w", name, pkgFilter, err)
		}
		fmt.Fprintf(w, "published %s\n", tag)
		return nil
	}

	published := 0
	for _, pp := range plan.Packages {
		if pp.SuggestedVersion == "" {
			continue
		}

		tag := format.Render(pp.SuggestedVersion, pp.Name)
		if err := b.Publish(ctx, tag); err != nil {
			return fmt.Errorf("%s: package %q: %w", name, pp.Name, err)
		}

		fmt.Fprintf(w, "published %s\n", tag)
		published++
	}

	if published == 0 {
		fmt.Fprintln(w, "nothing to publish: no packages have pending changes")
	}
	return nil
}

func suggestedVersionFor(plan *engine.Plan, pkgName string) string {
	for _, pp := range plan.Packages {
		if pp.Name == pkgName {
			return pp.SuggestedVersion
		}
	}
	return ""
}
