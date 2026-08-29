// Package backend defines the interface every git hosting adapter (GitHub,
// GitLab, Gitea, Forgejo) implements identically.
package backend

import (
	"context"

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
}
