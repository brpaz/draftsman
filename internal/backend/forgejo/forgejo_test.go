package forgejo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/backend/forgejo"
)

func TestUpsertDraft_CreatesWhenAbsent(t *testing.T) {
	var createBody map[string]any
	created := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/brpaz/draftsman/releases":
			assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/brpaz/draftsman/releases":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&createBody))
			created = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": 1, "tag_name": "v1.0.0", "draft": true}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := forgejo.New(server.URL, "brpaz", "draftsman", "test-token")
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
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/brpaz/draftsman/releases":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 42, "tag_name": "v1.0.0", "draft": true}]`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/repos/brpaz/draftsman/releases/42":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&patchBody))
			patched = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 42, "tag_name": "v1.0.0", "draft": true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/brpaz/draftsman/releases":
			posted = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := forgejo.New(server.URL, "brpaz", "draftsman", "test-token")
	err := client.UpsertDraft(context.Background(), backend.UpsertDraftRequest{
		Tag: "v1.0.0", Body: "## Bug Fixes\n- correct typo\n",
	})
	require.NoError(t, err)

	require.True(t, patched, "existing draft should be updated via PATCH")
	require.False(t, posted, "no new release should be created when one already exists")
	assert.Equal(t, "## Bug Fixes\n- correct typo\n", patchBody["body"])
}

func TestUpsertDraft_RefusesToModifyPublishedRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/brpaz/draftsman/releases":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 42, "tag_name": "v1.0.0", "draft": false}]`))
		default:
			t.Fatalf("unexpected request: %s %s — a published release must not be mutated", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := forgejo.New(server.URL, "brpaz", "draftsman", "test-token")
	err := client.UpsertDraft(context.Background(), backend.UpsertDraftRequest{Tag: "v1.0.0", Body: "notes"})
	require.Error(t, err)
}

func TestUpsertDraft_PaginatesUntilFound(t *testing.T) {
	patched := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/brpaz/draftsman/releases" && r.URL.Query().Get("page") == "1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 1, "tag_name": "v0.9.0", "draft": false}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/brpaz/draftsman/releases" && r.URL.Query().Get("page") == "2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 2, "tag_name": "v1.0.0", "draft": true}]`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/repos/brpaz/draftsman/releases/2":
			patched = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 2, "tag_name": "v1.0.0", "draft": true}`))
		default:
			t.Fatalf("unexpected request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	client := forgejo.New(server.URL, "brpaz", "draftsman", "test-token")
	err := client.UpsertDraft(context.Background(), backend.UpsertDraftRequest{Tag: "v1.0.0", Body: "notes"})
	require.NoError(t, err, "the matching draft on page 2 should be found and updated")
	require.True(t, patched)
}

func TestPublish_FlipsDraftToPublished(t *testing.T) {
	var patchBody map[string]any
	patched := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/brpaz/draftsman/releases":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 7, "tag_name": "v1.0.0", "draft": true}]`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/repos/brpaz/draftsman/releases/7":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&patchBody))
			patched = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 7, "tag_name": "v1.0.0", "draft": false}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := forgejo.New(server.URL, "brpaz", "draftsman", "test-token")
	err := client.Publish(context.Background(), "v1.0.0")
	require.NoError(t, err)

	require.True(t, patched)
	assert.Equal(t, false, patchBody["draft"])
}

func TestPublish_AlreadyPublishedIsIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/brpaz/draftsman/releases":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 7, "tag_name": "v1.0.0", "draft": false}]`))
		default:
			t.Fatalf("unexpected request: %s %s — publishing an already-published release must be a no-op", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := forgejo.New(server.URL, "brpaz", "draftsman", "test-token")
	err := client.Publish(context.Background(), "v1.0.0")
	require.NoError(t, err)
}

func TestPublish_NoDraftFoundIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := forgejo.New(server.URL, "brpaz", "draftsman", "test-token")
	err := client.Publish(context.Background(), "v1.0.0")
	require.Error(t, err)
}

// TestResolvePR_AlwaysUnsupported confirms ResolvePR never calls Forgejo's
// commits/{sha}/pull endpoint. That endpoint does exist on Forgejo (unlike
// Gitea, which has none at all) — the failing default handler here proves
// the adapter deliberately ignores it per ADR-0001's "confirmed unreliable"
// exclusion, not that it forgot to wire it up.
func TestResolvePR_AlwaysUnsupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("ResolvePR must not call Forgejo's commits/{sha}/pull endpoint — deliberately excluded as unreliable (ADR-0001), got %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := forgejo.New(server.URL, "brpaz", "draftsman", "test-token")
	ref, ok, err := client.ResolvePR(context.Background(), "abc123")
	require.NoError(t, err)
	require.False(t, ok)
	assert.Zero(t, ref)
}

func TestCommitURL_UsesBaseURLAsWebRoot(t *testing.T) {
	client := forgejo.New("https://codeberg.org/", "brpaz", "draftsman", "test-token")
	require.Equal(t, "https://codeberg.org/brpaz/draftsman/commit/abc123", client.CommitURL("abc123"))
}

func TestCompareURL_UsesBaseURLAsWebRoot(t *testing.T) {
	client := forgejo.New("https://codeberg.org/", "brpaz", "draftsman", "test-token")
	require.Equal(t, "https://codeberg.org/brpaz/draftsman/compare/v1.0.0...v1.1.0", client.CompareURL("v1.0.0", "v1.1.0"))
}
