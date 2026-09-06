// Package config loads and defaults a repo's .draftsman.yml.
package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	ModeSingle = "single"
	ModeMulti  = "multi"

	// StrategyDraft is the default release-strategy: a continuously-updated
	// Draft Release object on the backend, never touching repo files.
	StrategyDraft = "draft"
	// StrategyPR bumps a package's version file and CHANGELOG.md via a
	// reviewable release PR instead of a Draft Release — see
	// .scratch/pr-release-strategy/spec.md.
	StrategyPR = "pr"

	defaultSkipChangelogTrailer = "Skip-Changelog"
	defaultTagFormat            = "v{{version}}"

	// {{if .Name}} skips the package heading entirely when packages aren't
	// configured (the implicit single package has an empty Name), keeping
	// single-package output identical to before Packages existed. A
	// Package's own SuggestedVersion (multi mode) shows inline on its
	// heading; the top-level one (single mode) shows once, up front —
	// exactly one of the two is ever non-empty for a given Compute call.
	defaultTemplate = `{{if .SuggestedVersion}}# {{.SuggestedVersion}}

{{end}}{{range .Packages}}{{if .Name}}# {{.Name}}{{if .SuggestedVersion}} ({{.SuggestedVersion}}){{end}}
{{end}}{{range .Sections}}## {{.Name}}
{{range .Entries}}- {{if .Breaking}}**💥 BREAKING:** {{end}}{{.Description}}{{if .PR}} ({{if .PR.Link}}[#{{.PR.Number}}]({{.PR.Link}}){{else}}#{{.PR.Number}}{{end}}){{end}} by {{if .AuthorRef}}[@{{.AuthorRef.Login}}]({{.AuthorRef.ProfileURL}}){{else}}{{.Author}}{{end}} ({{if .CommitURL}}[{{.ShortSHA}}]({{.CommitURL}}){{else}}{{.ShortSHA}}{{end}})
{{end}}
{{end}}{{if .Name}}{{if .CompareURL}}
**Full Changelog**: {{.CompareURL}}
{{end}}{{else}}{{if $.CompareURL}}
**Full Changelog**: {{$.CompareURL}}
{{end}}{{end}}
{{end}}`
)

// Category maps a Conventional Commit type — optionally narrowed to a
// specific scope — to a changelog section name. A commit is matched
// against the configured Categories in order, first match wins; Scope
// empty matches any scope for that Type. This lets a scope-specific rule
// (e.g. type "fix", scope "security") take a commit before a broader
// type-only rule for the same Type, as long as it's listed first. Order in
// the config also determines section display order.
type Category struct {
	Type    string `yaml:"type"`
	Scope   string `yaml:"scope"`
	Section string `yaml:"section"`
}

// Package maps a path prefix to a monorepo package name. A commit is
// attributed to every Package whose Path prefixes one of its changed files.
type Package struct {
	Path string `yaml:"path"`
	Name string `yaml:"name"`
	// VersionFile overrides release-strategy: pr's auto-detected version
	// file for this Package, as a path relative to Path. Format (JSON/TOML/
	// plain text) is inferred from the extension (.json/.toml, anything
	// else treated like the plain-text VERSION fallback) — used when
	// auto-detection would pick the wrong file among several manifests in
	// the same directory, or the ecosystem isn't one of the built-in
	// detected formats.
	VersionFile string `yaml:"version-file"`
}

// Config is the fully-defaulted result of loading .draftsman.yml.
type Config struct {
	Mode string `yaml:"mode"`
	// ReleaseStrategy chooses between the continuous Draft Release model
	// (StrategyDraft, default) and the version-bump release PR model
	// (StrategyPR) — orthogonal to Mode: any combination of the two is
	// valid.
	ReleaseStrategy      string     `yaml:"release-strategy"`
	Categories           []Category `yaml:"categories"`
	Packages             []Package  `yaml:"packages"`
	SkipChangelogTrailer string     `yaml:"skip-changelog-trailer"`
	// TagFormat locates the previous release tag and, in multi mode
	// (ticket 06+), the {{package}} placeholder scopes it per Package. In
	// single mode {{package}} is accepted but unused.
	TagFormat string `yaml:"tag-format"`
	Template  string `yaml:"template"`
	// Footer controls whether draftsman appends its own attribution
	// footer to every rendered changelog body. A pointer so Load can tell
	// "not set in this repo's config" (nil, keep Default's true) apart
	// from an explicit "footer: false" (set, false) — a plain bool can't
	// make that distinction, since both read as the zero value.
	Footer *bool `yaml:"footer"`
}

// FooterEnabled reports whether c.Footer's attribution footer should be
// appended. A nil Footer (any Config not built through Default/Load, e.g.
// a test-constructed literal) defaults to true, same as Default's own
// value — callers never need to nil-check Footer themselves.
func (c *Config) FooterEnabled() bool {
	return c.Footer == nil || *c.Footer
}

// Default returns the built-in configuration used when no field is
// overridden — this is also what a repo with no .draftsman.yml gets.
func Default() *Config {
	footer := true
	return &Config{
		Mode:            ModeSingle,
		ReleaseStrategy: StrategyDraft,
		Categories: []Category{
			{Type: "feat", Section: "Features"},
			{Type: "fix", Section: "Bug Fixes"},
		},
		SkipChangelogTrailer: defaultSkipChangelogTrailer,
		TagFormat:            defaultTagFormat,
		Template:             defaultTemplate,
		Footer:               &footer,
	}
}

// Load reads path and applies any fields it sets on top of Default().
// A missing file is only an error when required is true — callers should
// pass required = true exactly when the path was explicitly requested
// (e.g. an explicit --config flag), so an absent default path silently
// falls back to defaults while an absent explicit path is a real error.
func Load(path string, required bool) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !required {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}

	var overrides Config
	if err := yaml.Unmarshal(data, &overrides); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}

	if overrides.Mode != "" {
		cfg.Mode = overrides.Mode
	}
	if overrides.ReleaseStrategy != "" {
		cfg.ReleaseStrategy = overrides.ReleaseStrategy
	}
	if overrides.SkipChangelogTrailer != "" {
		cfg.SkipChangelogTrailer = overrides.SkipChangelogTrailer
	}
	if overrides.TagFormat != "" {
		cfg.TagFormat = overrides.TagFormat
	}
	if overrides.Template != "" {
		cfg.Template = overrides.Template
	}
	if len(overrides.Categories) > 0 {
		// A full replace, not a merge: a config that cares enough to
		// reorder or remap categories lists everything it wants, in the
		// order it wants — a partial merge can't express reordering
		// unambiguously when only some types are overridden.
		cfg.Categories = overrides.Categories
	}
	if len(overrides.Packages) > 0 {
		cfg.Packages = overrides.Packages
	}
	if overrides.Footer != nil {
		cfg.Footer = overrides.Footer
	}

	return cfg, nil
}
