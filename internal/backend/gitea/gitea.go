// Package gitea implements backend.Backend against the Gitea REST API
// (api/v1), which is close enough to GitHub's release API in shape
// (tag_name/name/body/draft) that the adapter logic mirrors
// internal/backend/github — the differences are the base path, the
// Authorization header scheme, and list pagination's query param names.
package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/commit"
)

// maxListPages bounds how many pages of releases UpsertDraft will scan
// looking for an existing release matching a tag, so a repo with an
// unexpectedly huge release history fails loudly instead of looping forever.
const maxListPages = 20

// Client implements backend.Backend against a Gitea instance's REST API.
type Client struct {
	baseURL, owner, repo, token string
	httpClient                  *http.Client
}

var _ backend.Backend = (*Client)(nil)

// New returns a Client for owner/repo against the Gitea instance at
// baseURL (e.g. "https://gitea.example.com" — no trailing slash or
// "/api/v1" suffix needed, New adds it). Unlike GitHub's fixed
// api.github.com, Gitea is self-hosted, so baseURL is always required.
func New(baseURL, owner, repo, token string) *Client {
	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		owner:      owner,
		repo:       repo,
		token:      token,
		httpClient: http.DefaultClient,
	}
}

type release struct {
	ID      int64  `json:"id"`
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
}

// UpsertDraft implements backend.Backend.
func (c *Client) UpsertDraft(ctx context.Context, req backend.UpsertDraftRequest) error {
	existing, err := c.findReleaseByTag(ctx, req.Tag)
	if err != nil {
		return fmt.Errorf("finding existing release for tag %q: %w", req.Tag, err)
	}

	if existing == nil {
		return c.createDraft(ctx, req)
	}
	if !existing.Draft {
		return fmt.Errorf("release for tag %q already exists and is published, refusing to modify it", req.Tag)
	}
	return c.updateRelease(ctx, existing.ID, req)
}

// Publish implements backend.Backend: flips the draft release matching tag
// to published. As with GitHub, the underlying git tag doesn't exist until
// this point; leaving target_commitish unset in the PATCH means Gitea keeps
// whatever was set at draft-creation time (also left unset, which Gitea
// resolves to the default branch), so publish always tags the default
// branch's current state rather than a stale commitish.
func (c *Client) Publish(ctx context.Context, tag string) error {
	existing, err := c.findReleaseByTag(ctx, tag)
	if err != nil {
		return fmt.Errorf("finding release for tag %q: %w", tag, err)
	}
	if existing == nil {
		return fmt.Errorf("no draft release found for tag %q", tag)
	}
	if !existing.Draft {
		// Already published — publish is idempotent on repeated runs.
		return nil
	}
	return c.publishRelease(ctx, existing.ID)
}

// ResolvePR implements backend.Backend. Gitea has no commit→PR lookup
// endpoint (unlike GitHub's commits/{sha}/pulls) — per ADR-0001, only
// ticket 05's text extraction (the "Reviewed-on:" trailer) applies here, so
// this always reports "not supported" rather than attempting a lookup that
// doesn't exist.
func (c *Client) ResolvePR(_ context.Context, _ string) (commit.PRReference, bool, error) {
	return commit.PRReference{}, false, nil
}

// CommitURL implements backend.Backend, using baseURL as the instance's
// web root (same host as the API, just without the "/api/v1" prefix).
func (c *Client) CommitURL(sha string) string {
	return fmt.Sprintf("%s/%s/%s/commit/%s", c.baseURL, c.owner, c.repo, sha)
}

// ResolveAuthor implements backend.Backend. Unlike GitHub's "get a commit"
// endpoint, no Gitea endpoint returning a commit's linked account has been
// verified against a live instance — per ADR-0001, this always reports
// "not supported" rather than attempting an unverified lookup.
func (c *Client) ResolveAuthor(_ context.Context, _ string) (backend.AuthorReference, bool, error) {
	return backend.AuthorReference{}, false, nil
}

// findReleaseByTag looks for a release matching tag. Draft releases have
// no underlying git tag yet, so — same as the GitHub adapter — this lists
// releases and filters client-side rather than using a get-by-tag endpoint.
func (c *Client) findReleaseByTag(ctx context.Context, tag string) (*release, error) {
	for page := 1; page <= maxListPages; page++ {
		releases, err := c.listReleasesPage(ctx, page)
		if err != nil {
			return nil, err
		}
		if len(releases) == 0 {
			return nil, nil
		}
		for _, r := range releases {
			if r.TagName == tag {
				return &r, nil
			}
		}
	}
	return nil, fmt.Errorf("exceeded %d pages scanning releases for tag %q", maxListPages, tag)
}

func (c *Client) listReleasesPage(ctx context.Context, page int) ([]release, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/releases?limit=100&page=%d", c.baseURL, c.owner, c.repo, page)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, unexpectedStatus(resp)
	}

	var releases []release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decoding releases: %w", err)
	}
	return releases, nil
}

func (c *Client) createDraft(ctx context.Context, req backend.UpsertDraftRequest) error {
	payload, err := json.Marshal(map[string]any{
		"tag_name": req.Tag,
		"name":     releaseName(req),
		"body":     req.Body,
		"draft":    true,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/releases", c.baseURL, c.owner, c.repo)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	c.setHeaders(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return unexpectedStatus(resp)
	}
	return nil
}

func (c *Client) updateRelease(ctx context.Context, id int64, req backend.UpsertDraftRequest) error {
	payload, err := json.Marshal(map[string]any{
		"name": releaseName(req),
		"body": req.Body,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/releases/%d", c.baseURL, c.owner, c.repo, id)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	c.setHeaders(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return unexpectedStatus(resp)
	}
	return nil
}

func (c *Client) publishRelease(ctx context.Context, id int64) error {
	payload, err := json.Marshal(map[string]any{"draft": false})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/releases/%d", c.baseURL, c.owner, c.repo, id)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	c.setHeaders(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return unexpectedStatus(resp)
	}
	return nil
}

func releaseName(req backend.UpsertDraftRequest) string {
	if req.Name != "" {
		return req.Name
	}
	return req.Tag
}

// setHeaders sets the Gitea token auth scheme — "Authorization: token
// <token>", distinct from GitHub's "Bearer <token>".
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")
}

func unexpectedStatus(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
}
