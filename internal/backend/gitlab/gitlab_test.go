package gitlab_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/backend/gitlab"
)

const projectPath = "/api/v4/projects/brpaz%2Fdraftsman"

func TestUpsertDraft_CreatesWhenAbsent(t *testing.T) {
	var createBody map[string]any
	created := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == projectPath+"/releases/v1.0.0":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.EscapedPath() == projectPath:
			assert.Equal(t, "test-token", r.Header.Get("PRIVATE-TOKEN"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"default_branch": "main"}`))
		case r.Method == http.MethodPost && r.URL.EscapedPath() == projectPath+"/releases":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&createBody))
			created = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"tag_name": "v1.0.0", "upcoming_release": true}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.EscapedPath())
		}
	}))
	defer server.Close()

	client := gitlab.New(server.URL, "brpaz", "draftsman", "test-token")
	err := client.UpsertDraft(context.Background(), backend.UpsertDraftRequest{
		Tag: "v1.0.0", Body: "## Features\n- add login page\n",
	})
	require.NoError(t, err)

	require.True(t, created)
	assert.Equal(t, "v1.0.0", createBody["tag_name"])
	assert.Equal(t, "main", createBody["ref"], "ref is required to create the tag when it doesn't exist yet")
	assert.Equal(t, "## Features\n- add login page\n", createBody["description"])
	assert.Equal(t, "9999-01-01T00:00:00Z", createBody["released_at"], "far-future released_at is what fakes the draft state")
}

func TestUpsertDraft_UpdatesWhenPresentAndUpcoming(t *testing.T) {
	var putBody map[string]any
	updated := false
	posted := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == projectPath+"/releases/v1.0.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name": "v1.0.0", "upcoming_release": true}`))
		case r.Method == http.MethodPut && r.URL.EscapedPath() == projectPath+"/releases/v1.0.0":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&putBody))
			updated = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"tag_name": "v1.0.0", "upcoming_release": true}`))
		case r.Method == http.MethodPost:
			posted = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.EscapedPath())
		}
	}))
	defer server.Close()

	client := gitlab.New(server.URL, "brpaz", "draftsman", "test-token")
	err := client.UpsertDraft(context.Background(), backend.UpsertDraftRequest{
		Tag: "v1.0.0", Body: "## Bug Fixes\n- correct typo\n",
	})
	require.NoError(t, err)

	require.True(t, updated, "existing upcoming release should be updated via PUT")
	require.False(t, posted, "no new release should be created when one already exists")
	assert.Equal(t, "## Bug Fixes\n- correct typo\n", putBody["description"])
	assert.NotContains(t, putBody, "released_at", "update must not touch released_at — that's Publish's job")
}

func TestUpsertDraft_RefusesToModifyPublishedRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == projectPath+"/releases/v1.0.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name": "v1.0.0", "upcoming_release": false}`))
		default:
			t.Fatalf("unexpected request: %s %s — a published release must not be mutated", r.Method, r.URL.EscapedPath())
		}
	}))
	defer server.Close()

	client := gitlab.New(server.URL, "brpaz", "draftsman", "test-token")
	err := client.UpsertDraft(context.Background(), backend.UpsertDraftRequest{Tag: "v1.0.0", Body: "notes"})
	require.Error(t, err)
}

func TestPublish_FlipsUpcomingToPublished(t *testing.T) {
	var putBody map[string]any
	updated := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == projectPath+"/releases/v1.0.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name": "v1.0.0", "upcoming_release": true}`))
		case r.Method == http.MethodPut && r.URL.EscapedPath() == projectPath+"/releases/v1.0.0":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&putBody))
			updated = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"tag_name": "v1.0.0", "upcoming_release": false}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.EscapedPath())
		}
	}))
	defer server.Close()

	client := gitlab.New(server.URL, "brpaz", "draftsman", "test-token")
	err := client.Publish(context.Background(), "v1.0.0")
	require.NoError(t, err)

	require.True(t, updated)
	assert.NotEmpty(t, putBody["released_at"])
	assert.NotEqual(t, "9999-01-01T00:00:00Z", putBody["released_at"])
}

func TestPublish_AlreadyPublishedIsIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == projectPath+"/releases/v1.0.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name": "v1.0.0", "upcoming_release": false}`))
		default:
			t.Fatalf("unexpected request: %s %s — publishing an already-published release must be a no-op", r.Method, r.URL.EscapedPath())
		}
	}))
	defer server.Close()

	client := gitlab.New(server.URL, "brpaz", "draftsman", "test-token")
	err := client.Publish(context.Background(), "v1.0.0")
	require.NoError(t, err)
}

func TestPublish_NoDraftFoundIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := gitlab.New(server.URL, "brpaz", "draftsman", "test-token")
	err := client.Publish(context.Background(), "v1.0.0")
	require.Error(t, err)
}

func TestResolvePR_Found(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, projectPath+"/repository/commits/abc123/merge_requests", r.URL.EscapedPath())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"iid": 42, "web_url": "https://gitlab.com/brpaz/draftsman/-/merge_requests/42"}]`))
	}))
	defer server.Close()

	client := gitlab.New(server.URL, "brpaz", "draftsman", "test-token")
	ref, ok, err := client.ResolvePR(context.Background(), "abc123")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, 42, ref.Number)
	assert.Equal(t, "https://gitlab.com/brpaz/draftsman/-/merge_requests/42", ref.Link)
}

func TestResolvePR_NoneAssociated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := gitlab.New(server.URL, "brpaz", "draftsman", "test-token")
	_, ok, err := client.ResolvePR(context.Background(), "abc123")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestResolveAuthor_AlwaysUnsupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("ResolveAuthor must not make any HTTP call — no verified GitLab endpoint links a commit to an account, got %s %s", r.Method, r.URL.EscapedPath())
	}))
	defer server.Close()

	client := gitlab.New(server.URL, "brpaz", "draftsman", "test-token")
	ref, ok, err := client.ResolveAuthor(context.Background(), "abc123")
	require.NoError(t, err)
	require.False(t, ok)
	assert.Zero(t, ref)
}

func TestCommitURL_UsesBaseURLAsWebRoot(t *testing.T) {
	client := gitlab.New("https://gitlab.example.com/", "brpaz", "draftsman", "test-token")
	require.Equal(t, "https://gitlab.example.com/brpaz/draftsman/-/commit/abc123", client.CommitURL("abc123"))
}

func TestCommitURL_NestedGroup(t *testing.T) {
	client := gitlab.New("https://gitlab.com", "group", "subgroup/project", "test-token")
	require.Equal(t, "https://gitlab.com/group/subgroup/project/-/commit/abc123", client.CommitURL("abc123"))
}

func TestCompareURL_UsesBaseURLAsWebRoot(t *testing.T) {
	client := gitlab.New("https://gitlab.example.com/", "brpaz", "draftsman", "test-token")
	require.Equal(t, "https://gitlab.example.com/brpaz/draftsman/-/compare/v1.0.0...v1.1.0", client.CompareURL("v1.0.0", "v1.1.0"))
}
