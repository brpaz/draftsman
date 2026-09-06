// Package publish implements the "publish" command: promote a draft
// release to published, tagging it on the backend.
package publish

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/changelog"
	"github.com/brpaz/draftsman/internal/commands/shared"
	"github.com/brpaz/draftsman/internal/config"
	"github.com/brpaz/draftsman/internal/engine"
	"github.com/brpaz/draftsman/internal/version"
	"github.com/brpaz/draftsman/internal/versionfile"
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

	if cfg.ReleaseStrategy == config.StrategyPR {
		if cmd.IsSet("version") {
			return fmt.Errorf("%s: --version doesn't apply under release-strategy: pr (the version is whatever was merged in the release PR)", name)
		}
		return runPR(ctx, ".", cfg, b, cmd.String("package"), cmd.Writer)
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

// runPR publishes release-strategy: pr's already-merged release(s). Unlike
// the draft-strategy path above, this never calls engine.Compute — the
// merged CHANGELOG.md is trusted verbatim as the release body (no
// recomputation/drift risk), and the version comes from the package's own
// version file, the same one draft bumped (falling back to the
// changelog entry's own heading only for a Go module's VersionFileNoOp
// case, which has no version file to read at all).
func runPR(ctx context.Context, repoPath string, cfg *config.Config, b backend.Backend, pkgFilter string, w io.Writer) error {
	packages, err := packagesToPublish(cfg, pkgFilter)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	format, err := version.ParseFormat(cfg.TagFormat)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	published := 0
	for _, pkg := range packages {
		ok, err := publishPackage(ctx, repoPath, b, format, pkg, w)
		if err != nil {
			return fmt.Errorf("%s: package %q: %w", name, pkg.Name, err)
		}
		if ok {
			published++
		}
	}

	if published == 0 {
		if pkgFilter != "" {
			return fmt.Errorf("%s: package %q has nothing to publish (no CHANGELOG.md entry found)", name, pkgFilter)
		}
		fmt.Fprintln(w, "nothing to publish: no packages have a merged release to publish")
	}
	return nil
}

// packagesToPublish resolves which config.Package entries runPR should
// consider: every configured Package in multi mode (or just pkgFilter's,
// if given), or the single implicit root package in single mode (where
// pkgFilter makes no sense and is rejected).
func packagesToPublish(cfg *config.Config, pkgFilter string) ([]config.Package, error) {
	if cfg.Mode != config.ModeMulti {
		if pkgFilter != "" {
			return nil, fmt.Errorf("--package is only meaningful in multi mode")
		}
		return []config.Package{{}}, nil
	}

	if pkgFilter == "" {
		return cfg.Packages, nil
	}
	for _, pkg := range cfg.Packages {
		if pkg.Name == pkgFilter {
			return []config.Package{pkg}, nil
		}
	}
	return nil, fmt.Errorf("unknown package %q", pkgFilter)
}

// publishPackage reads pkg's CHANGELOG.md top entry and, unless
// VersionFileNoOp, its version file's current value, then calls
// Backend.CreateRelease. ok is false — not an error — when pkg's
// CHANGELOG.md has no entry yet (nothing merged for it so far).
func publishPackage(ctx context.Context, repoPath string, b backend.Backend, format *version.Format, pkg config.Package, w io.Writer) (bool, error) {
	changelogPath := filepath.Join(repoPath, pkg.Path, "CHANGELOG.md")
	content, err := os.ReadFile(changelogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading CHANGELOG.md: %w", err)
	}

	changelogVersion, body, ok := changelog.TopEntry(content)
	if !ok {
		return false, nil
	}

	ver := changelogVersion
	target, hasVersionFile, err := versionfile.Detect(repoPath, pkg)
	if err != nil {
		return false, fmt.Errorf("detecting version file: %w", err)
	}
	if hasVersionFile {
		fileContent, err := os.ReadFile(filepath.Join(repoPath, target.Path))
		if err != nil {
			return false, fmt.Errorf("reading %s: %w", target.Path, err)
		}
		ver, err = versionfile.Read(target, fileContent)
		if err != nil {
			return false, fmt.Errorf("reading version from %s: %w", target.Path, err)
		}
	}

	tag := format.Render(ver, pkg.Name)
	if err := b.CreateRelease(ctx, tag, "", body); err != nil {
		return false, err
	}

	fmt.Fprintf(w, "published %s\n", tag)
	return true, nil
}
