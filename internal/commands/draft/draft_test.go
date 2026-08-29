package draft

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/commit"
	"github.com/brpaz/draftsman/internal/config"
	"github.com/brpaz/draftsman/internal/engine"
	"github.com/brpaz/draftsman/internal/version"
)

// fakeBackend is a minimal backend.Backend double recording UpsertDraft
// calls, so multi-package wiring can be verified without a real HTTP
// server — the GitHub adapter's own HTTP mechanics are already covered by
// internal/backend/github's httptest suite.
type fakeBackend struct {
	upserts []backend.UpsertDraftRequest
}

func (f *fakeBackend) UpsertDraft(_ context.Context, req backend.UpsertDraftRequest) error {
	f.upserts = append(f.upserts, req)
	return nil
}
func (f *fakeBackend) Publish(context.Context, string) error { return nil }
func (f *fakeBackend) ResolvePR(context.Context, string) (commit.PRReference, bool, error) {
	return commit.PRReference{}, false, nil
}

func multiPlan() *engine.Plan {
	return &engine.Plan{
		Packages: []engine.PackagePlan{
			{
				Name:             "api",
				SuggestedVersion: "1.0.1",
				Sections:         []engine.Section{{Name: "Bug Fixes", Entries: []engine.Entry{{Description: "handle nil"}}}},
			},
			{
				Name:             "web",
				SuggestedVersion: "1.1.0",
				Sections:         []engine.Section{{Name: "Features", Entries: []engine.Entry{{Description: "dark mode"}}}},
			},
		},
	}
}

func multiFormat(t *testing.T) *version.Format {
	t.Helper()
	f, err := version.ParseFormat("{{package}}-v{{version}}")
	require.NoError(t, err)
	return f
}

func TestRunMulti_UpsertsOneDraftPerPendingPackage(t *testing.T) {
	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runMulti(context.Background(), config.Default(), fb, multiPlan(), multiFormat(t), "", &out)
	require.NoError(t, err)

	require.Len(t, fb.upserts, 2)

	byTag := map[string]backend.UpsertDraftRequest{}
	for _, u := range fb.upserts {
		byTag[u.Tag] = u
	}

	require.Contains(t, byTag, "api-v1.0.1")
	assert.Contains(t, byTag["api-v1.0.1"].Body, "handle nil")
	assert.NotContains(t, byTag["api-v1.0.1"].Body, "dark mode", "api's draft body must not leak web's entries")

	require.Contains(t, byTag, "web-v1.1.0")
	assert.Contains(t, byTag["web-v1.1.0"].Body, "dark mode")
	assert.NotContains(t, byTag["web-v1.1.0"].Body, "handle nil", "web's draft body must not leak api's entries")
}

func TestRunMulti_PackageFlagScopesToOnePackage(t *testing.T) {
	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runMulti(context.Background(), config.Default(), fb, multiPlan(), multiFormat(t), "web", &out)
	require.NoError(t, err)

	require.Len(t, fb.upserts, 1, "--package=web must not touch api")
	assert.Equal(t, "web-v1.1.0", fb.upserts[0].Tag)
}

func TestRunMulti_UnknownPackageFlagIsError(t *testing.T) {
	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runMulti(context.Background(), config.Default(), fb, multiPlan(), multiFormat(t), "missing", &out)
	require.Error(t, err)
	assert.Empty(t, fb.upserts)
}

func TestRunMulti_NoPendingPackagesIsNotAnError(t *testing.T) {
	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runMulti(context.Background(), config.Default(), fb, &engine.Plan{}, multiFormat(t), "", &out)
	require.NoError(t, err)
	assert.Empty(t, fb.upserts)
	assert.Contains(t, out.String(), "nothing to release")
}
