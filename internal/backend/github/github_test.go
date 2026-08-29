package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/backend/github"
)

func TestUpsertDraft_CreatesWhenAbsent(t *testing.T) {
	var createBody map[string]any
	created := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/brpaz/draftsman/releases":
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/brpaz/draftsman/releases":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&createBody))
			created = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": 1, "tag_name": "v1.0.0", "draft": true}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	err := client.UpsertDraft(context.Background(), backend.UpsertDraftRequest{
		Tag: "v1.0.0", Body: "## Features\n- add login page\n",
	})
	require.NoError(t, err)

	require.True(t, created)
	assert.Equal(t, "v1.0.0", createBody["tag_name"])
	assert.Equal(t, true, createBody["draft"])
	assert.Equal(t, "## Features\n- add login page\n", createBody["body"])
}

func TestUpsertDraft_UpdatesWhenPresent(t *testing.T) {
	var patchBody map[string]any
	patched := false
	posted := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/brpaz/draftsman/releases":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 42, "tag_name": "v1.0.0", "draft": true}]`))
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/brpaz/draftsman/releases/42":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&patchBody))
			patched = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 42, "tag_name": "v1.0.0", "draft": true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/brpaz/draftsman/releases":
			posted = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	err := client.UpsertDraft(context.Background(), backend.UpsertDraftRequest{
		Tag: "v1.0.0", Body: "## Bug Fixes\n- correct typo\n",
	})
	require.NoError(t, err)

	require.True(t, patched, "existing draft should be updated via PATCH")
	require.False(t, posted, "no new release should be created when one already exists")
	assert.Equal(t, "## Bug Fixes\n- correct typo\n", patchBody["body"])
}

func TestUpsertDraft_FindsExistingDraftByName_WhenTagNameIsGitHubsUntaggedPlaceholder(t *testing.T) {
	var patchBody map[string]any
	patched := false
	posted := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/brpaz/draftsman/releases":
			w.Header().Set("Content-Type", "application/json")
			// GitHub rewrites tag_name to a random "untagged-<hash>"
			// placeholder for a draft whose tag doesn't exist yet — name
			// stays what was requested, so that's the only reliable match.
			_, _ = w.Write([]byte(`[{"id": 42, "tag_name": "untagged-abc123", "name": "v1.0.0", "draft": true}]`))
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/brpaz/draftsman/releases/42":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&patchBody))
			patched = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 42, "tag_name": "untagged-abc123", "name": "v1.0.0", "draft": true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/brpaz/draftsman/releases":
			posted = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	err := client.UpsertDraft(context.Background(), backend.UpsertDraftRequest{
		Tag: "v1.0.0", Body: "## Bug Fixes\n- correct typo\n",
	})
	require.NoError(t, err)

	require.True(t, patched, "the existing draft should be found by name and updated, not duplicated")
	require.False(t, posted, "no new release should be created when a matching draft already exists")
}

func TestUpsertDraft_RefusesToModifyPublishedRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/brpaz/draftsman/releases":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 42, "tag_name": "v1.0.0", "draft": false}]`))
		default:
			t.Fatalf("unexpected request: %s %s — a published release must not be mutated", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	err := client.UpsertDraft(context.Background(), backend.UpsertDraftRequest{Tag: "v1.0.0", Body: "notes"})
	require.Error(t, err)
}

func TestUpsertDraft_PaginatesUntilFound(t *testing.T) {
	patched := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/brpaz/draftsman/releases" && r.URL.Query().Get("page") == "1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 1, "tag_name": "v0.9.0", "draft": false}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/brpaz/draftsman/releases" && r.URL.Query().Get("page") == "2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 2, "tag_name": "v1.0.0", "draft": true}]`))
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/brpaz/draftsman/releases/2":
			patched = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 2, "tag_name": "v1.0.0", "draft": true}`))
		default:
			t.Fatalf("unexpected request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	err := client.UpsertDraft(context.Background(), backend.UpsertDraftRequest{Tag: "v1.0.0", Body: "notes"})
	require.NoError(t, err, "the matching draft on page 2 should be found and updated")
	require.True(t, patched)
}

func TestPublish_FlipsDraftToPublished(t *testing.T) {
	var patchBody map[string]any
	patched := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/brpaz/draftsman/releases":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 7, "tag_name": "v1.0.0", "draft": true}]`))
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/brpaz/draftsman/releases/7":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&patchBody))
			patched = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 7, "tag_name": "v1.0.0", "draft": false}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	err := client.Publish(context.Background(), "v1.0.0")
	require.NoError(t, err)

	require.True(t, patched)
	assert.Equal(t, false, patchBody["draft"])
}

func TestPublish_AlreadyPublishedIsIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/brpaz/draftsman/releases":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 7, "tag_name": "v1.0.0", "draft": false}]`))
		default:
			t.Fatalf("unexpected request: %s %s — publishing an already-published release must be a no-op", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	err := client.Publish(context.Background(), "v1.0.0")
	require.NoError(t, err)
}

func TestPublish_NoDraftFoundIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	err := client.Publish(context.Background(), "v1.0.0")
	require.Error(t, err)
}

func TestResolvePR_Found(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/repos/brpaz/draftsman/commits/abc123/pulls", r.URL.Path)
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"number": 42, "html_url": "https://github.com/brpaz/draftsman/pull/42"}]`))
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	ref, ok, err := client.ResolvePR(context.Background(), "abc123")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, 42, ref.Number)
	assert.Equal(t, "https://github.com/brpaz/draftsman/pull/42", ref.Link)
}

func TestResolvePR_NoneAssociated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	_, ok, err := client.ResolvePR(context.Background(), "abc123")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestResolvePR_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	_, ok, err := client.ResolvePR(context.Background(), "abc123")
	require.NoError(t, err, "a 404 is a normal 'no such commit' result, not an error")
	require.False(t, ok)
}

func TestCommitURL_IsGitHubWebLink(t *testing.T) {
	// The web URL is always github.com, regardless of the API base URL
	// (WithBaseURL only redirects API calls, e.g. to a test server).
	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL("https://example.invalid"))
	require.Equal(t, "https://github.com/brpaz/draftsman/commit/abc123", client.CommitURL("abc123"))
}

func TestResolveAuthor_LinkedAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/repos/brpaz/draftsman/commits/abc123", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"author": map[string]any{"login": "brpaz", "html_url": "https://github.com/brpaz"},
		})
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	ref, ok, err := client.ResolveAuthor(context.Background(), "abc123")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, backend.AuthorReference{Login: "brpaz", ProfileURL: "https://github.com/brpaz"}, ref)
}

func TestResolveAuthor_NoLinkedAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"author": nil})
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	ref, ok, err := client.ResolveAuthor(context.Background(), "abc123")
	require.NoError(t, err)
	require.False(t, ok)
	assert.Zero(t, ref)
}

func TestResolveAuthor_CommitNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := github.New("brpaz", "draftsman", "test-token", github.WithBaseURL(server.URL))
	_, ok, err := client.ResolveAuthor(context.Background(), "abc123")
	require.NoError(t, err, "a 404 is a normal 'no such commit' result, not an error")
	require.False(t, ok)
}
