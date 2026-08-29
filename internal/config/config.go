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
{{range .Entries}}- {{.Description}}{{if .PR}} ({{if .PR.Link}}[#{{.PR.Number}}]({{.PR.Link}}){{else}}#{{.PR.Number}}{{end}}){{end}}
{{end}}
{{end}}{{end}}`
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
}

// Config is the fully-defaulted result of loading .draftsman.yml.
type Config struct {
	Mode                 string     `yaml:"mode"`
	Categories           []Category `yaml:"categories"`
	Packages             []Package  `yaml:"packages"`
	SkipChangelogTrailer string     `yaml:"skip-changelog-trailer"`
	// TagFormat locates the previous release tag and, in multi mode
	// (ticket 06+), the {{package}} placeholder scopes it per Package. In
	// single mode {{package}} is accepted but unused.
	TagFormat string `yaml:"tag-format"`
	Template  string `yaml:"template"`
}

// Default returns the built-in configuration used when no field is
// overridden — this is also what a repo with no .draftsman.yml gets.
func Default() *Config {
	return &Config{
		Mode: ModeSingle,
		Categories: []Category{
			{Type: "feat", Section: "Features"},
			{Type: "fix", Section: "Bug Fixes"},
		},
		SkipChangelogTrailer: defaultSkipChangelogTrailer,
		TagFormat:            defaultTagFormat,
		Template:             defaultTemplate,
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

	return cfg, nil
}
