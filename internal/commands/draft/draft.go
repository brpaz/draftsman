// Package draft implements the "draft" command: compute entries since the
// last release and upsert the draft release(s) on the backend.
package draft

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/commands/shared"
	"github.com/brpaz/draftsman/internal/config"
	"github.com/brpaz/draftsman/internal/engine"
	"github.com/brpaz/draftsman/internal/git"
	"github.com/brpaz/draftsman/internal/releasepr"
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

	if cfg.ReleaseStrategy == config.StrategyPR {
		return runPR(ctx, ".", cfg, b, plan, cmd.Writer)
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

// runPR opens or updates one release PR per package with a pending release
// (single mode has exactly one implicit package), per release-strategy: pr.
// A branch whose current HEAD wasn't authored by draftsman's own bot
// identity is left alone (see backOffForHumanEdit) rather than
// force-updated, so a human's manual review edit on the branch survives
// across runs.
func runPR(ctx context.Context, repoPath string, cfg *config.Config, b backend.Backend, plan *engine.Plan, w io.Writer) error {
	filePlans, err := releasepr.Compute(repoPath, cfg, plan, time.Now().UTC().Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if len(filePlans) == 0 {
		fmt.Fprintln(w, "nothing to release: no packages have commits since their last release that warrant one")
		return nil
	}

	base, err := git.CurrentBranch(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("%s: determining base branch: %w", name, err)
	}
	remote := b.GitRemoteURL()

	for _, fp := range filePlans {
		branch := releaseBranchName(fp.Package)
		title := releasePRTitle(fp.Package, fp.NewVersion)

		skip, err := backOffForHumanEdit(ctx, repoPath, remote, branch)
		if err != nil {
			return fmt.Errorf("%s: checking branch %q for manual edits: %w", name, branch, err)
		}
		if skip {
			fmt.Fprintf(w, "skipping %s: release branch has manual edits, leaving it alone\n", releasePRLabel(fp.Package))
			continue
		}

		var changes []git.FileChange
		if !fp.VersionFileNoOp {
			changes = append(changes, git.FileChange{Path: fp.VersionFilePath, Content: fp.VersionFileContent})
		}
		changes = append(changes, git.FileChange{Path: fp.ChangelogPath, Content: []byte(fp.ChangelogEntry)})

		if _, err := git.PushBranch(ctx, repoPath, remote, branch, base, changes, title); err != nil {
			return fmt.Errorf("%s: pushing release branch for package %q: %w", name, fp.Package, err)
		}

		pr, err := b.UpsertReleasePR(ctx, backend.UpsertReleasePRRequest{
			Branch: branch, Base: base, Title: title, Body: fp.ChangelogEntry,
		})
		if err != nil {
			return fmt.Errorf("%s: opening release PR for package %q: %w", name, fp.Package, err)
		}

		fmt.Fprintf(w, "release PR upserted for %s: %s\n", releasePRLabel(fp.Package), pr.URL)
	}

	return nil
}

// backOffForHumanEdit reports whether branch already exists on remote with
// a HEAD commit not authored by draftsman's own bot identity — a human has
// pushed a manual edit to the release PR since the last run, and it should
// be left alone rather than force-updated (ticket 04 of
// .scratch/pr-release-strategy/spec.md). A branch that doesn't exist yet,
// or whose HEAD is still bot-authored, is always safe to update.
func backOffForHumanEdit(ctx context.Context, repoPath, remote, branch string) (bool, error) {
	authorEmail, exists, err := git.RemoteBranchAuthorEmail(ctx, repoPath, remote, branch)
	if err != nil {
		return false, err
	}
	return exists && authorEmail != git.BotEmail, nil
}

// releaseBranchName is stable and deterministic per package (not
// versioned), so re-running against the same pending release updates the
// same branch/PR rather than opening a new one each time.
func releaseBranchName(pkg string) string {
	if pkg == "" {
		return "draftsman-release"
	}
	return "draftsman-release--" + pkg
}

func releasePRTitle(pkg, version string) string {
	if pkg == "" {
		return fmt.Sprintf("chore(release): %s", version)
	}
	return fmt.Sprintf("chore(release): %s %s", pkg, version)
}

func releasePRLabel(pkg string) string {
	if pkg == "" {
		return "(root)"
	}
	return pkg
}
