// Package releasepr computes, for release-strategy: pr, the version-file
// bump and CHANGELOG.md update each package with a pending release would
// get. It never touches disk beyond reading current file content — writing
// the result and pushing a branch is a later stage (see
// .scratch/pr-release-strategy/spec.md).
package releasepr

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/brpaz/draftsman/internal/changelog"
	"github.com/brpaz/draftsman/internal/config"
	"github.com/brpaz/draftsman/internal/engine"
	"github.com/brpaz/draftsman/internal/versionfile"
)

// FilePlan is one package's computed release-strategy: pr changes.
type FilePlan struct {
	Package string

	// VersionFileNoOp is true for a detected Go module: no file carries an
	// in-repo version to bump (see versionfile.Detect). VersionFilePath,
	// OldVersion and NewVersion are all empty in that case.
	VersionFileNoOp bool
	// VersionFilePath is relative to the repository root.
	VersionFilePath string
	OldVersion      string
	NewVersion      string
	// VersionFileContent is the version file's full new content, ready to
	// write as-is — nil when VersionFileNoOp.
	VersionFileContent []byte

	// ChangelogPath is relative to the repository root.
	ChangelogPath string
	// ChangelogEntry is the full CHANGELOG.md content after prepending
	// this release's entry, ready to write as-is.
	ChangelogEntry string
}

// Compute returns one FilePlan per package in plan with a pending release
// (a non-empty suggested version), reading each package's current version
// file and CHANGELOG.md from repoPath. date is injected rather than read
// from the clock internally, so callers get deterministic output.
func Compute(repoPath string, cfg *config.Config, plan *engine.Plan, date string) ([]FilePlan, error) {
	if cfg.Mode == config.ModeMulti {
		var out []FilePlan
		for _, pp := range plan.Packages {
			if pp.SuggestedVersion == "" {
				continue
			}
			fp, err := computeOne(repoPath, cfg, packageConfig(cfg.Packages, pp.Name), pp.Sections, pp.SuggestedVersion, date)
			if err != nil {
				return nil, fmt.Errorf("package %q: %w", pp.Name, err)
			}
			out = append(out, fp)
		}
		return out, nil
	}

	if plan.SuggestedVersion == "" {
		return nil, nil
	}

	// Single mode: one implicit package at the repo root. buildPackagePlans
	// produces exactly one entry in plan.Packages whenever there's anything
	// release-worthy, holding the sections to render into the changelog
	// entry (Package.Name/Path are irrelevant here — cfg.Packages is empty
	// in single mode).
	var sections []engine.Section
	if len(plan.Packages) > 0 {
		sections = plan.Packages[0].Sections
	}

	fp, err := computeOne(repoPath, cfg, config.Package{}, sections, plan.SuggestedVersion, date)
	if err != nil {
		return nil, err
	}
	return []FilePlan{fp}, nil
}

func packageConfig(packages []config.Package, name string) config.Package {
	for _, p := range packages {
		if p.Name == name {
			return p
		}
	}
	return config.Package{Name: name}
}

func computeOne(repoPath string, cfg *config.Config, pkg config.Package, sections []engine.Section, newVersion, date string) (FilePlan, error) {
	fp := FilePlan{Package: pkg.Name}

	target, ok, err := versionfile.Detect(repoPath, pkg)
	if err != nil {
		return FilePlan{}, fmt.Errorf("detecting version file: %w", err)
	}
	if !ok {
		fp.VersionFileNoOp = true
	} else {
		content, err := readFile(repoPath, target.Path)
		if err != nil {
			return FilePlan{}, err
		}
		newContent, oldVersion, err := versionfile.Bump(target, content, newVersion)
		if err != nil {
			return FilePlan{}, fmt.Errorf("bumping %s: %w", target.Path, err)
		}

		fp.VersionFilePath = target.Path
		fp.VersionFileContent = newContent
		fp.OldVersion = oldVersion
		fp.NewVersion = newVersion
	}

	// Rendered with no top-level SuggestedVersion and an unnamed Package,
	// so the template emits only the sectioned entries — no duplicate
	// version heading (that's changelog.Prepend's "## version - date"
	// line) and no package heading (empty Name, same convention
	// buildPackagePlans relies on for the implicit single package).
	body, err := engine.RenderPlan(cfg.Template, &engine.Plan{Packages: []engine.PackagePlan{{Sections: sections}}}, false)
	if err != nil {
		return FilePlan{}, fmt.Errorf("rendering changelog entry: %w", err)
	}

	changelogPath := filepath.Join(pkg.Path, "CHANGELOG.md")
	existing, err := readFile(repoPath, changelogPath)
	if err != nil {
		return FilePlan{}, err
	}

	fp.ChangelogPath = changelogPath
	fp.ChangelogEntry = string(changelog.Prepend(existing, newVersion, date, body))

	return fp, nil
}

// readFile returns nil, nil for a missing file — every caller here treats
// "doesn't exist yet" as valid input (a VERSION file or CHANGELOG.md about
// to be created for the first time), not an error.
func readFile(repoPath, relPath string) ([]byte, error) {
	content, err := os.ReadFile(filepath.Join(repoPath, relPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", relPath, err)
	}
	return content, nil
}
