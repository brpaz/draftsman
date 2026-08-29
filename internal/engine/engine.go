// Package engine computes a release Plan from a repository's commit history.
package engine

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/commit"
	"github.com/brpaz/draftsman/internal/config"
	"github.com/brpaz/draftsman/internal/git"
	"github.com/brpaz/draftsman/internal/version"
)

// Entry is one changelog line, derived from a single parsed commit.
type Entry struct {
	SHA         string
	Type        string
	Scope       string
	Description string
	// PR is the commit's resolved PR Reference, or nil when none was
	// found — enrichment only, never required (ADR-0001, ADR-0003).
	PR *commit.PRReference
}

// Section is a named group of Entries (e.g. "Features", "Bug Fixes").
type Section struct {
	Name    string
	Entries []Entry
}

// PackagePlan is one Package's sectioned Entries. Name is empty when
// packages aren't configured — an implicit single package standing in for
// the whole repo.
//
// PreviousVersion/SuggestedVersion are only populated in multi mode
// (ADR-0004), where each Package is versioned and tagged independently; in
// single mode they stay empty and the repo-wide Plan fields apply instead.
type PackagePlan struct {
	Name             string
	Sections         []Section
	PreviousVersion  string
	SuggestedVersion string
}

// Plan is the computed result of a release: every affected Package's
// sectioned Entries, plus the changelog body rendered through the
// configured template. PreviousVersion/SuggestedVersion are the repo-wide
// version (single mode only — see PackagePlan for multi mode).
type Plan struct {
	Packages         []PackagePlan
	PreviousVersion  string
	SuggestedVersion string
	Rendered         string
}

const otherSection = "Other"

// firstReleaseVersion is what a SuggestedVersion becomes when there is no
// previous version to increment from — a fixed bootstrap default rather
// than treating a first release as a semver milestone.
const firstReleaseVersion = "0.1.0"

// Compute parses repoPath's commits as Conventional Commits, drops any
// carrying cfg's skip-changelog trailer, buckets the rest into cfg's
// sections, and computes a suggested next version from the
// highest-severity type seen (breaking > feat > fix). In single mode
// (default) this happens once, repo-wide, bounded by the last tag matching
// cfg.TagFormat, with Packages used only to section Entries (ADR-0004). In
// multi mode it happens independently per configured Package, each bounded
// by its own last tag (cfg.TagFormat with {{package}} resolved).
//
// be is optional (nil is fine) — when a commit's PR Reference can't be
// extracted from its text (ADR-0001), be.ResolvePR is tried as a
// best-effort fallback. Whether that fallback does anything is entirely up
// to the adapter: only GitHub's actually looks anything up, so passing a
// Gitea/Forgejo Backend (or none) transparently yields the same
// text-extraction-only behavior.
func Compute(ctx context.Context, repoPath string, cfg *config.Config, be backend.Backend) (*Plan, error) {
	format, err := version.ParseFormat(cfg.TagFormat)
	if err != nil {
		return nil, fmt.Errorf("invalid tag-format: %w", err)
	}

	sectionOrder := resolveSectionOrder(cfg.Categories)

	var plan *Plan
	if cfg.Mode == config.ModeMulti {
		plan, err = computeMulti(ctx, repoPath, cfg, be, format, sectionOrder)
	} else {
		plan, err = computeSingle(ctx, repoPath, cfg, be, format, sectionOrder)
	}
	if err != nil {
		return nil, err
	}

	rendered, err := RenderPlan(cfg.Template, plan)
	if err != nil {
		return nil, err
	}
	plan.Rendered = rendered

	return plan, nil
}

// RenderPlan executes tmplText (a config's Template) against plan, producing
// changelog body text. Compute uses this for the full repo-wide/multi Plan;
// multi-mode draft/publish commands reuse it to render an isolated body for
// a single Package's own draft release, by wrapping that one PackagePlan in
// a synthetic *Plan.
func RenderPlan(tmplText string, plan *Plan) (string, error) {
	tmpl, err := template.New("release").Parse(tmplText)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, plan); err != nil {
		return "", fmt.Errorf("rendering template: %w", err)
	}
	return buf.String(), nil
}

// computeSingle computes one repo-wide Plan, sectioned by Package (if
// configured) but versioned as a single unit.
func computeSingle(ctx context.Context, repoPath string, cfg *config.Config, be backend.Backend, format *version.Format, sectionOrder []string) (*Plan, error) {
	since, previous, hasPrevious, err := latestMatchingTag(ctx, repoPath, format)
	if err != nil {
		return nil, fmt.Errorf("finding previous release tag: %w", err)
	}

	commits, err := git.Log(ctx, repoPath, since)
	if err != nil {
		return nil, fmt.Errorf("reading commit history: %w", err)
	}

	// packageName -> sectionName -> Entries
	byPackage := make(map[string]map[string][]Entry)
	bump := version.BumpNone

	for _, c := range commits {
		pe, ok := processCommit(ctx, be, c, cfg)
		if !ok {
			continue
		}
		bump = version.Max(bump, pe.Bump)

		packages, err := affectedPackages(ctx, repoPath, c.SHA, cfg.Packages)
		if err != nil {
			return nil, fmt.Errorf("resolving packages for %s: %w", c.SHA, err)
		}

		for _, pkg := range packages {
			if byPackage[pkg] == nil {
				byPackage[pkg] = make(map[string][]Entry)
			}
			byPackage[pkg][pe.Section] = append(byPackage[pkg][pe.Section], pe.Entry)
		}
	}

	plan := &Plan{Packages: buildPackagePlans(cfg.Packages, byPackage, sectionOrder)}

	if hasPrevious {
		plan.PreviousVersion = previous.String()
	}
	switch {
	case bump == version.BumpNone:
		// No release-worthy Entries — leave SuggestedVersion empty.
	case hasPrevious:
		plan.SuggestedVersion = previous.Apply(bump).String()
	default:
		plan.SuggestedVersion = firstReleaseVersion
	}

	return plan, nil
}

// computeMulti computes one independent Plan per configured Package: its
// own tag pattern, its own "since last release" range, its own Entries
// (filtered to commits touching that Package), and its own suggested
// version. Packages with no release-worthy Entries in their own range are
// omitted, same as computeSingle's empty-section skip.
func computeMulti(ctx context.Context, repoPath string, cfg *config.Config, be backend.Backend, format *version.Format, sectionOrder []string) (*Plan, error) {
	plan := &Plan{}

	for _, pkg := range cfg.Packages {
		pkgFormat, err := format.ForPackage(pkg.Name)
		if err != nil {
			return nil, fmt.Errorf("resolving tag format for package %q: %w", pkg.Name, err)
		}

		since, previous, hasPrevious, err := latestMatchingTag(ctx, repoPath, pkgFormat)
		if err != nil {
			return nil, fmt.Errorf("finding previous release tag for package %q: %w", pkg.Name, err)
		}

		commits, err := git.Log(ctx, repoPath, since)
		if err != nil {
			return nil, fmt.Errorf("reading commit history for package %q: %w", pkg.Name, err)
		}

		bySection := make(map[string][]Entry)
		bump := version.BumpNone

		for _, c := range commits {
			pe, ok := processCommit(ctx, be, c, cfg)
			if !ok {
				continue
			}

			files, err := git.ChangedFiles(ctx, repoPath, c.SHA)
			if err != nil {
				return nil, fmt.Errorf("resolving changed files for %s: %w", c.SHA, err)
			}
			if !anyMatchesPackage(files, pkg.Path) {
				continue
			}

			bump = version.Max(bump, pe.Bump)
			bySection[pe.Section] = append(bySection[pe.Section], pe.Entry)
		}

		var sections []Section
		for _, name := range sectionOrder {
			entries := bySection[name]
			if len(entries) == 0 {
				continue
			}
			sections = append(sections, Section{Name: name, Entries: entries})
		}
		if len(sections) == 0 {
			continue
		}

		pp := PackagePlan{Name: pkg.Name, Sections: sections}
		if hasPrevious {
			pp.PreviousVersion = previous.String()
		}
		switch {
		case bump == version.BumpNone:
		case hasPrevious:
			pp.SuggestedVersion = previous.Apply(bump).String()
		default:
			pp.SuggestedVersion = firstReleaseVersion
		}

		plan.Packages = append(plan.Packages, pp)
	}

	return plan, nil
}

// processedCommit is what one commit contributes to a Plan, once parsed.
type processedCommit struct {
	Section string
	Entry   Entry
	Bump    version.Bump
}

// processCommit parses c as a Conventional Commit, resolves its section and
// PR Reference, and reports the SemVer severity it warrants. ok is false
// when c isn't a Conventional Commit or carries cfg's skip-changelog
// trailer or a "[skip changelog]" tag — callers drop it entirely.
//
// PR Reference resolution tries commit.ExtractPRReference (text) first; if
// that finds nothing and be is non-nil, be.ResolvePR is tried as a
// best-effort fallback (ADR-0001). A ResolvePR error doesn't fail the
// commit — enrichment is optional, never required (ADR-0003), so a
// transient API problem degrades to "no reference" rather than aborting
// the whole release computation.
func processCommit(ctx context.Context, be backend.Backend, c git.Commit, cfg *config.Config) (processedCommit, bool) {
	parsed, ok := commit.Parse(c.Message)
	if !ok {
		return processedCommit{}, false
	}
	trailers := commit.Trailers(c.Message)
	if strings.EqualFold(trailers[cfg.SkipChangelogTrailer], "true") || commit.HasSkipChangelogTag(c.Message) {
		return processedCommit{}, false
	}

	section, known := matchCategory(cfg.Categories, parsed.Type, parsed.Scope)
	if !known {
		section = otherSection
	}

	entry := Entry{SHA: c.SHA, Type: parsed.Type, Scope: parsed.Scope, Description: parsed.Description}

	pr, ok := commit.ExtractPRReference(c.Message)
	if !ok && be != nil {
		if resolved, found, err := be.ResolvePR(ctx, c.SHA); err == nil && found {
			pr, ok = resolved, true
		}
	}
	if ok {
		entry.PR = &pr
		// GitHub's squash suffix is part of the Conventional Commit
		// description text itself (e.g. "add login page (#42)") — strip
		// it so the template's own PR rendering doesn't duplicate it. Only
		// text-extracted references can appear in Description at all, so
		// this is a no-op (CutSuffix just won't match) for API-resolved ones.
		suffix := fmt.Sprintf("(#%d)", pr.Number)
		if trimmed, cut := strings.CutSuffix(entry.Description, suffix); cut {
			entry.Description = strings.TrimSpace(trimmed)
		}
	}

	return processedCommit{Section: section, Entry: entry, Bump: bumpFor(parsed)}, true
}

// bumpFor returns the SemVer severity a single parsed commit warrants.
// Tied to the literal Conventional Commit type/breaking marker, independent
// of how cfg.Categories relabels types for display.
func bumpFor(parsed commit.ParsedCommit) version.Bump {
	switch {
	case parsed.Breaking:
		return version.BumpMajor
	case parsed.Type == "feat":
		return version.BumpMinor
	case parsed.Type == "fix", parsed.Type == "chore":
		return version.BumpPatch
	default:
		return version.BumpNone
	}
}

// latestMatchingTag finds the highest-SemVer tag reachable from HEAD that
// matches format, returning its name (for git.Log's since) and version.
// hasPrevious is false when no tag matches — the caller then uses the
// repo's full history and firstReleaseVersion.
func latestMatchingTag(ctx context.Context, repoPath string, format *version.Format) (tag string, v version.SemVer, hasPrevious bool, err error) {
	tags, err := git.Tags(ctx, repoPath)
	if err != nil {
		return "", version.SemVer{}, false, err
	}

	for _, candidate := range tags {
		candidateVersion, ok := format.Match(candidate)
		if !ok {
			continue
		}
		if !hasPrevious || candidateVersion.GreaterThan(v) {
			tag, v, hasPrevious = candidate, candidateVersion, true
		}
	}

	return tag, v, hasPrevious, nil
}

// resolveSectionOrder returns the section display order: each configured
// Category's Section, first-seen (declaration order), with "Other"
// appended last if not already present.
func resolveSectionOrder(categories []config.Category) []string {
	order := make([]string, 0, len(categories)+1)
	seen := make(map[string]bool, len(categories)+1)

	for _, c := range categories {
		if !seen[c.Section] {
			seen[c.Section] = true
			order = append(order, c.Section)
		}
	}
	if !seen[otherSection] {
		order = append(order, otherSection)
	}

	return order
}

// matchCategory finds the first configured Category matching commitType and
// commitScope — first match wins, in config order. A Category with an
// empty Scope matches any commitScope for that Type, so a scope-specific
// rule only takes precedence over a broader type-only rule for the same
// Type when it's listed first (see config.Category's doc comment).
func matchCategory(categories []config.Category, commitType, commitScope string) (section string, known bool) {
	for _, c := range categories {
		if c.Type != commitType {
			continue
		}
		if c.Scope != "" && c.Scope != commitScope {
			continue
		}
		return c.Section, true
	}
	return "", false
}

// affectedPackages returns the (deduplicated) set of configured package
// names whose path prefixes match one of sha's changed files. With no
// packages configured, every commit belongs to the single implicit package
// (empty name). With packages configured, a commit matching none of them
// is unmapped — it belongs to no package and is dropped.
func affectedPackages(ctx context.Context, repoPath, sha string, packages []config.Package) ([]string, error) {
	if len(packages) == 0 {
		return []string{""}, nil
	}

	files, err := git.ChangedFiles(ctx, repoPath, sha)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, pkg := range packages {
		if anyMatchesPackage(files, pkg.Path) {
			names = append(names, pkg.Name)
		}
	}
	return names, nil
}

func anyMatchesPackage(files []string, path string) bool {
	for _, f := range files {
		if matchesPackage(f, path) {
			return true
		}
	}
	return false
}

func matchesPackage(file, path string) bool {
	path = strings.TrimSuffix(path, "/")
	return file == path || strings.HasPrefix(file, path+"/")
}

// buildPackagePlans assembles the final, ordered []PackagePlan: config
// declaration order when packages are configured (single implicit package
// otherwise), each with its sections in sectionOrder, skipping any package
// or section left with zero Entries.
func buildPackagePlans(packages []config.Package, byPackage map[string]map[string][]Entry, sectionOrder []string) []PackagePlan {
	names := []string{""}
	if len(packages) > 0 {
		names = make([]string, len(packages))
		for i, pkg := range packages {
			names[i] = pkg.Name
		}
	}

	var plans []PackagePlan
	for _, name := range names {
		sections := byPackage[name]
		var pkgSections []Section
		for _, sectionName := range sectionOrder {
			entries := sections[sectionName]
			if len(entries) == 0 {
				continue
			}
			pkgSections = append(pkgSections, Section{Name: sectionName, Entries: entries})
		}
		if len(pkgSections) == 0 {
			continue
		}
		plans = append(plans, PackagePlan{Name: name, Sections: pkgSections})
	}
	return plans
}
