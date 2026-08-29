// Package gitlab implements backend.Backend against the GitLab Releases
// API. GitLab has no draft/hidden release state like GitHub's — UpsertDraft
// fakes one via GitLab's "Upcoming Release" mechanism: a release whose
// released_at is set to a future date is flagged upcoming_release=true and
// badged "Upcoming Release" in the UI. Publish flips released_at to the
// current time, clearing the flag.
//
// Unlike GitHub/Gitea/Forgejo, this means the underlying git tag is
// created as soon as the "draft" exists: GitLab's create-release endpoint
// requires ref up front to create the tag when it doesn't exist yet —
// there's no way to create a release without also creating its tag.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	neturl "net/url"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/commit"
)

// DefaultBaseURL is gitlab.com's root — used when the caller doesn't
// override it for a self-hosted instance.
const DefaultBaseURL = "https://gitlab.com"

// farFutureReleaseDate is released_at for a freshly-created "draft" — far
// enough out that it reads as unambiguously not yet released.
const farFutureReleaseDate = "9999-01-01T00:00:00Z"

// Client implements backend.Backend against a GitLab instance's REST API
// (api/v4) — gitlab.com by default, or a self-hosted instance via baseURL.
type Client struct {
	baseURL, projectPath, token string
	httpClient                  *http.Client
	// defaultBranch is lazily resolved and memoized — only createDraft
	// needs it (see package doc), and at most once per Client.
	defaultBranch string
}

var _ backend.Backend = (*Client)(nil)

// New returns a Client for owner/repo against the GitLab instance at
// baseURL (e.g. "https://gitlab.com", or a self-hosted root — no trailing
// slash or "/api/v4" suffix needed, New adds it). repo may itself contain
// "/" for a nested group (e.g. "subgroup/project").
func New(baseURL, owner, repo, token string) *Client {
	return &Client{
		baseURL:     strings.TrimSuffix(baseURL, "/"),
		projectPath: owner + "/" + repo,
		token:       token,
		httpClient:  http.DefaultClient,
	}
}

type release struct {
	TagName         string `json:"tag_name"`
	UpcomingRelease bool   `json:"upcoming_release"`
}

// UpsertDraft implements backend.Backend, faking a "draft" via GitLab's
// Upcoming Release mechanism (see package doc).
func (c *Client) UpsertDraft(ctx context.Context, req backend.UpsertDraftRequest) error {
	existing, err := c.findRelease(ctx, req.Tag)
	if err != nil {
		return fmt.Errorf("finding existing release for tag %q: %w", req.Tag, err)
	}

	if existing == nil {
		return c.createDraft(ctx, req)
	}
	if !existing.UpcomingRelease {
		return fmt.Errorf("release for tag %q already exists and is published, refusing to modify it", req.Tag)
	}
	return c.updateRelease(ctx, req)
}

// Publish implements backend.Backend: flips the draft's released_at to
// now, clearing upcoming_release.
func (c *Client) Publish(ctx context.Context, tag string) error {
	existing, err := c.findRelease(ctx, tag)
	if err != nil {
		return fmt.Errorf("finding release for tag %q: %w", tag, err)
	}
	if existing == nil {
		return fmt.Errorf("no draft release found for tag %q", tag)
	}
	if !existing.UpcomingRelease {
		// Already published — publish is idempotent on repeated runs.
		return nil
	}
	return c.setReleasedAt(ctx, tag, time.Now().UTC().Format(time.RFC3339))
}

// ResolvePR implements backend.Backend using GitLab's "list merge requests
// associated with a commit" endpoint — a documented, reliable lookup
// (unlike Forgejo's equivalent, flagged unreliable per ADR-0001).
func (c *Client) ResolvePR(ctx context.Context, sha string) (commit.PRReference, bool, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/repository/commits/%s/merge_requests", c.baseURL, c.encodedProject(), sha)

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

	var mrs []struct {
		IID    int    `json:"iid"`
		WebURL string `json:"web_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mrs); err != nil {
		return commit.PRReference{}, false, fmt.Errorf("decoding merge requests for %s: %w", sha, err)
	}
	if len(mrs) == 0 {
		return commit.PRReference{}, false, nil
	}

	return commit.PRReference{Number: mrs[0].IID, Link: mrs[0].WebURL}, true, nil
}

// CommitURL implements backend.Backend.
func (c *Client) CommitURL(sha string) string {
	return fmt.Sprintf("%s/%s/-/commit/%s", c.baseURL, c.projectPath, sha)
}

// CompareURL implements backend.Backend.
func (c *Client) CompareURL(from, to string) string {
	return fmt.Sprintf("%s/%s/-/compare/%s...%s", c.baseURL, c.projectPath, from, to)
}

// ResolveAuthor implements backend.Backend. GitLab's "get a single commit"
// endpoint returns only the raw git author_name/author_email, not a linked
// account — per ADR-0001, this always reports "not supported" rather than
// guessing one from an unverified lookup.
func (c *Client) ResolveAuthor(_ context.Context, _ string) (backend.AuthorReference, bool, error) {
	return backend.AuthorReference{}, false, nil
}

func (c *Client) findRelease(ctx context.Context, tag string) (*release, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/releases/%s", c.baseURL, c.encodedProject(), neturl.PathEscape(tag))

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

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, unexpectedStatus(resp)
	}

	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decoding release for tag %q: %w", tag, err)
	}
	return &r, nil
}

// resolveDefaultBranch returns the project's default branch, memoized.
func (c *Client) resolveDefaultBranch(ctx context.Context) (string, error) {
	if c.defaultBranch != "" {
		return c.defaultBranch, nil
	}

	url := fmt.Sprintf("%s/api/v4/projects/%s", c.baseURL, c.encodedProject())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", unexpectedStatus(resp)
	}

	var project struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return "", fmt.Errorf("decoding project: %w", err)
	}

	c.defaultBranch = project.DefaultBranch
	return c.defaultBranch, nil
}

func (c *Client) createDraft(ctx context.Context, req backend.UpsertDraftRequest) error {
	branch, err := c.resolveDefaultBranch(ctx)
	if err != nil {
		return fmt.Errorf("resolving default branch: %w", err)
	}

	payload, err := json.Marshal(map[string]any{
		"tag_name":    req.Tag,
		"ref":         branch,
		"name":        releaseName(req),
		"description": req.Body,
		"released_at": farFutureReleaseDate,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v4/projects/%s/releases", c.baseURL, c.encodedProject())
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

func (c *Client) updateRelease(ctx context.Context, req backend.UpsertDraftRequest) error {
	payload, err := json.Marshal(map[string]any{
		"name":        releaseName(req),
		"description": req.Body,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v4/projects/%s/releases/%s", c.baseURL, c.encodedProject(), neturl.PathEscape(req.Tag))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
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

func (c *Client) setReleasedAt(ctx context.Context, tag, releasedAt string) error {
	payload, err := json.Marshal(map[string]any{"released_at": releasedAt})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v4/projects/%s/releases/%s", c.baseURL, c.encodedProject(), neturl.PathEscape(tag))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
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

func (c *Client) encodedProject() string {
	return neturl.PathEscape(c.projectPath)
}

func releaseName(req backend.UpsertDraftRequest) string {
	if req.Name != "" {
		return req.Name
	}
	return req.Tag
}

// setHeaders sets GitLab's personal/project access token auth header.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Accept", "application/json")
}

func unexpectedStatus(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
}
