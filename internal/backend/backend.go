// Package backend defines the interface every git hosting adapter (GitHub,
// Gitea, Forgejo) implements identically.
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
}
