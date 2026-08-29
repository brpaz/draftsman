// Package shared provides flags common to multiple draftsman commands.
package shared

import "github.com/urfave/cli/v3"

const DefaultConfigPath = ".draftsman.yml"

// ConfigFlag is the path to the repo's .draftsman.yml.
func ConfigFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:  "config",
		Usage: "path to the draftsman config file",
		Value: DefaultConfigPath,
	}
}

// BackendFlag selects which git backend to talk to. Required when set to
// true (draft/publish need to reach the backend API); optional for preview,
// where it only enables best-effort PR enrichment.
func BackendFlag(required bool) *cli.StringFlag {
	return &cli.StringFlag{
		Name:     "backend",
		Usage:    "git backend: github, gitlab, gitea, or forgejo",
		Required: required,
		Sources:  cli.EnvVars("DRAFTSMAN_BACKEND"),
	}
}

// TokenFlag is the backend API token. Required alongside BackendFlag.
func TokenFlag(required bool) *cli.StringFlag {
	return &cli.StringFlag{
		Name:     "token",
		Usage:    "backend API token",
		Required: required,
		Sources:  cli.EnvVars("DRAFTSMAN_TOKEN"),
	}
}

// PackageFlag scopes an operation to a single package in multi mode. Unused
// in single mode.
func PackageFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:  "package",
		Usage: "package name to scope this operation to (multi mode only)",
	}
}

// RepoFlag identifies the target repository as "owner/repo". Required
// alongside BackendFlag/TokenFlag for any command that talks to a backend
// API. GITHUB_REPOSITORY and CI_PROJECT_PATH are GitHub Actions' and
// GitLab CI's own auto-injected env vars, in the same "owner/repo" form
// (CI_PROJECT_PATH includes any nested group), so either works as a source
// with no extra setup.
func RepoFlag(required bool) *cli.StringFlag {
	return &cli.StringFlag{
		Name:     "repo",
		Usage:    "target repository, in owner/repo form",
		Required: required,
		Sources:  cli.EnvVars("DRAFTSMAN_REPO", "GITHUB_REPOSITORY", "CI_PROJECT_PATH"),
	}
}

// BaseURLFlag is the git hosting instance's API base URL. GitHub ignores
// it (fixed api.github.com); self-hosted backends (gitea, forgejo) require
// it — ResolveBackend enforces that, since it can't be expressed as a
// static per-flag Required (whether it's required depends on --backend's
// value, known only at runtime). GitLab defaults to gitlab.com when unset,
// same as GitHub, but accepts an override for a self-hosted instance.
func BaseURLFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:    "base-url",
		Usage:   "backend API base URL (required for gitea/forgejo, optional for gitlab, e.g. https://gitea.example.com)",
		Sources: cli.EnvVars("DRAFTSMAN_BASE_URL"),
	}
}
