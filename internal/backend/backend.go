// Package backend defines the interface every git hosting adapter (GitHub,
// GitLab, Gitea, Forgejo) implements identically.
package backend

import (
	"context"
	"fmt"
	"strings"

	"github.com/brpaz/draftsman/internal/commit"
)

// UpsertDraftRequest is what's needed to create or update the draft
// release for one tag.
type UpsertDraftRequest struct {
	Tag  string
	Name string // release title; adapters default to Tag when empty
	Body string // rendered changelog body (engine.Plan.Rendered)
}

// AuthorReference is a commit author's linked account on the backend,
// resolved via a live API call — the git commit's author name/email alone
// carries no such account (ADR-0001's rationale for PRReference applies
// identically here: link only what a reliable source confirms).
type AuthorReference struct {
	Login      string
	ProfileURL string
}

// UpsertReleasePRRequest is what's needed to open or update a
// release-strategy: pr release PR (see .scratch/pr-release-strategy/spec.md)
// for a branch that's already been pushed.
type UpsertReleasePRRequest struct {
	Branch string // head branch, already pushed
	Base   string // base branch — the repo's default branch
	Title  string
	Body   string
}

// ReleasePR identifies the PR UpsertReleasePR created or found.
type ReleasePR struct {
	Number int
	URL    string
}

// Backend is implemented identically by every git hosting adapter.
type Backend interface {
	// UpsertDraft creates a draft release for req.Tag if none exists, or
	// updates its body if one does — idempotent. It errors rather than
	// silently mutating a release that already exists for req.Tag and is
	// published (not a draft).
	UpsertDraft(ctx context.Context, req UpsertDraftRequest) error

	// Publish promotes the draft release for tag to published.
	Publish(ctx context.Context, tag string) error

	// ResolvePR looks up the PR associated with sha via a live API call.
	// ok is false when unsupported by this backend (see ADR-0001) or when
	// no PR was found.
	ResolvePR(ctx context.Context, sha string) (ref commit.PRReference, ok bool, err error)

	// CommitURL returns the web URL for viewing sha on this backend's
	// hosting UI. Pure string formatting from the repo coordinates the
	// adapter was constructed with — no API call, always succeeds.
	CommitURL(sha string) string

	// CompareURL returns the web URL for diffing from..to (tag or ref
	// names) on this backend's hosting UI. Pure string formatting, like
	// CommitURL — no API call, always succeeds.
	CompareURL(from, to string) string

	// ResolveAuthor looks up the account linked to sha's commit author via
	// a live API call. ok is false when unsupported by this backend or
	// when the commit's author has no linked account (e.g. a git author
	// email not tied to one) — callers fall back to the plain git author
	// name rather than guessing an account (ADR-0001).
	ResolveAuthor(ctx context.Context, sha string) (ref AuthorReference, ok bool, err error)

	// UpsertReleasePR creates a PR for req.Branch against req.Base if none
	// is open yet, or updates an existing open one's title/body —
	// idempotent, the release-strategy: pr counterpart to UpsertDraft.
	UpsertReleasePR(ctx context.Context, req UpsertReleasePRRequest) (ReleasePR, error)

	// GitRemoteURL returns a git remote URL for this repo with credentials
	// embedded, suitable for a local `git push` (see internal/git.PushBranch)
	// — pure string formatting from the repo coordinates and token the
	// adapter was constructed with, like CommitURL/CompareURL: no API call,
	// always succeeds.
	GitRemoteURL() string
}

// FormatGitRemoteURL builds an authenticated git remote URL from webBaseURL
// (e.g. "https://github.com", or a self-hosted forge's root — the same
// value each adapter already uses for its web-UI links) and path (the
// "owner/repo" — or, for GitLab, a possibly-nested project path). Shared by
// every adapter's GitRemoteURL so the credential-embedding format lives in
// one place.
func FormatGitRemoteURL(webBaseURL, username, token, path string) string {
	scheme, host, ok := strings.Cut(webBaseURL, "://")
	if !ok {
		scheme, host = "https", webBaseURL
	}
	return fmt.Sprintf("%s://%s:%s@%s/%s.git", scheme, username, token, host, path)
}
