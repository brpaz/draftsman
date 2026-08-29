// Package github implements backend.Backend against the GitHub REST API.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/commit"
)

const defaultBaseURL = "https://api.github.com"

// maxListPages bounds how many pages of releases UpsertDraft will scan
// looking for an existing release matching a tag, so a repo with an
// unexpectedly huge release history fails loudly instead of looping forever.
const maxListPages = 20

// Client implements backend.Backend against the GitHub REST API.
type Client struct {
	owner, repo, token string
	baseURL            string
	httpClient         *http.Client
}

var _ backend.Backend = (*Client)(nil)

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL — tests point this at an httptest
// server instead of the real api.github.com.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// New returns a Client for owner/repo, authenticated with token.
func New(owner, repo, token string, opts ...Option) *Client {
	c := &Client{
		owner:      owner,
		repo:       repo,
		token:      token,
		baseURL:    defaultBaseURL,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type release struct {
	ID      int64  `json:"id"`
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
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

// Publish implements backend.Backend. It flips the draft release matching
// tag to published. GitHub creates the underlying tag at this point (drafts
// have none yet), pointed at target_commitish — left unset here, so it
// defaults to the repository's default branch HEAD at publish time, exactly
// what the caller expects "publish" to mean.
//
// This must be two separate PATCH requests, not one combined payload —
// confirmed against the live API (see brpaz/draftsman's own v0.2.0
// release). A draft's stored tag_name is GitHub's own "untagged-<hash>"
// placeholder (see findReleaseByTag) until a real tag exists; sending
// draft:false and tag_name together in one request still creates the tag
// under the placeholder, silently ignoring the resupplied tag_name for
// that transaction. Setting tag_name first, in its own request while
// still a draft, then flipping draft:false in a second request, produces
// the correct tag every time.
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

	if err := c.patchRelease(ctx, existing.ID, map[string]any{"tag_name": tag}); err != nil {
		return fmt.Errorf("setting tag_name before publish: %w", err)
	}
	return c.patchRelease(ctx, existing.ID, map[string]any{"draft": false})
}

func (c *Client) patchRelease(ctx context.Context, id int64, fields map[string]any) error {
	payload, err := json.Marshal(fields)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/releases/%d", c.baseURL, c.owner, c.repo, id)
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

// ResolvePR implements backend.Backend using GitHub's "list pull requests
// associated with a commit" endpoint — reliable on GitHub (unlike Gitea,
// which has no equivalent, or Forgejo, whose equivalent is unreliable; see
// ADR-0001, where those adapters' ResolvePR always returns ok=false).
func (c *Client) ResolvePR(ctx context.Context, sha string) (commit.PRReference, bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s/pulls", c.baseURL, c.owner, c.repo, sha)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return commit.PRReference{}, false, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return commit.PRReference{}, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return commit.PRReference{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return commit.PRReference{}, false, unexpectedStatus(resp)
	}

	var pulls []struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pulls); err != nil {
		return commit.PRReference{}, false, fmt.Errorf("decoding pulls for %s: %w", sha, err)
	}
	if len(pulls) == 0 {
		return commit.PRReference{}, false, nil
	}

	return commit.PRReference{Number: pulls[0].Number, Link: pulls[0].HTMLURL}, true, nil
}

// CommitURL implements backend.Backend. GitHub's web UI is always
// github.com regardless of API base URL (only api.github.com is
// supported — no Enterprise base-URL override exists for this adapter).
func (c *Client) CommitURL(sha string) string {
	return fmt.Sprintf("https://github.com/%s/%s/commit/%s", c.owner, c.repo, sha)
}

// CompareURL implements backend.Backend.
func (c *Client) CompareURL(from, to string) string {
	return fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s", c.owner, c.repo, from, to)
}

// ResolveAuthor implements backend.Backend using GitHub's "get a commit"
// endpoint, whose "author" field is the linked GitHub account for the
// commit's git author email — null when that email isn't tied to any
// account, in which case ok is false and callers fall back to the plain
// git author name (ADR-0001).
func (c *Client) ResolveAuthor(ctx context.Context, sha string) (backend.AuthorReference, bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s", c.baseURL, c.owner, c.repo, sha)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return backend.AuthorReference{}, false, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return backend.AuthorReference{}, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return backend.AuthorReference{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return backend.AuthorReference{}, false, unexpectedStatus(resp)
	}

	var payload struct {
		Author *struct {
			Login   string `json:"login"`
			HTMLURL string `json:"html_url"`
		} `json:"author"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return backend.AuthorReference{}, false, fmt.Errorf("decoding commit %s: %w", sha, err)
	}
	if payload.Author == nil {
		return backend.AuthorReference{}, false, nil
	}

	return backend.AuthorReference{Login: payload.Author.Login, ProfileURL: payload.Author.HTMLURL}, true, nil
}

// findReleaseByTag looks for a release matching tag. GitHub's "get release
// by tag" endpoint doesn't reliably return draft releases (they have no
// underlying git tag until published), so this lists releases instead and
// filters client-side.
//
// A draft release's tag_name is NOT what was requested at creation time —
// until the underlying git tag actually exists (i.e. until publish), GitHub
// silently rewrites it to a random "untagged-<hash>" placeholder, so
// matching on tag_name alone never finds an existing draft and UpsertDraft
// creates a new one on every call instead of reusing it. Name isn't
// mangled this way, and createDraft/updateRelease always set it to req.Tag
// (releaseName falls back to Tag when no explicit Name is given, and
// nothing in this codebase supplies one), so a draft is also matched by
// Draft==true && Name==tag as a fallback. Once the release is published
// (a real tag exists), tag_name reverts to the real value and the primary
// match applies again.
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
			if r.TagName == tag || (r.Draft && r.Name == tag) {
				return &r, nil
			}
		}
	}
	return nil, fmt.Errorf("exceeded %d pages scanning releases for tag %q", maxListPages, tag)
}

func (c *Client) listReleasesPage(ctx context.Context, page int) ([]release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100&page=%d", c.baseURL, c.owner, c.repo, page)

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

	url := fmt.Sprintf("%s/repos/%s/%s/releases", c.baseURL, c.owner, c.repo)
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
	return c.patchRelease(ctx, id, map[string]any{
		"name": releaseName(req),
		"body": req.Body,
	})
}

func releaseName(req backend.UpsertDraftRequest) string {
	if req.Name != "" {
		return req.Name
	}
	return req.Tag
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

func unexpectedStatus(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
}
