package shared

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/backend/forgejo"
	"github.com/brpaz/draftsman/internal/backend/gitea"
	"github.com/brpaz/draftsman/internal/backend/github"
)

// ResolveBackend constructs a Backend from cmd's --backend/--token/--repo
// flags. When required is true (draft/publish, where these flags are
// already CLI-required), missing pieces or an unsupported backend are
// errors. When required is false (preview, where backend access is only
// optional PR-Reference enrichment — ADR-0001), anything missing or
// unsupported just means no enrichment: a nil Backend, no error.
func ResolveBackend(cmd *cli.Command, required bool) (backend.Backend, error) {
	name, token, repo := cmd.String("backend"), cmd.String("token"), cmd.String("repo")
	if name == "" || token == "" || repo == "" {
		if required {
			return nil, fmt.Errorf("--backend, --token, and --repo are all required")
		}
		return nil, nil
	}

	owner, repoName, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || repoName == "" {
		if required {
			return nil, fmt.Errorf("--repo must be in owner/repo form, got %q", repo)
		}
		return nil, nil
	}

	switch name {
	case "github":
		return github.New(owner, repoName, token), nil
	case "gitea":
		baseURL := cmd.String("base-url")
		if baseURL == "" {
			if required {
				return nil, fmt.Errorf("--base-url is required for --backend gitea")
			}
			return nil, nil
		}
		return gitea.New(baseURL, owner, repoName, token), nil
	case "forgejo":
		baseURL := cmd.String("base-url")
		if baseURL == "" {
			if required {
				return nil, fmt.Errorf("--base-url is required for --backend forgejo")
			}
			return nil, nil
		}
		return forgejo.New(baseURL, owner, repoName, token), nil
	default:
		if required {
			return nil, fmt.Errorf("unsupported backend %q (github, gitea, and forgejo are implemented)", name)
		}
		return nil, nil
	}
}
