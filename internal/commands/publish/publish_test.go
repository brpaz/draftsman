package publish

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/commit"
	"github.com/brpaz/draftsman/internal/engine"
	"github.com/brpaz/draftsman/internal/version"
)

// fakeBackend is a minimal backend.Backend double recording Publish calls,
// so multi-package wiring can be verified without a real HTTP server — the
// GitHub adapter's own HTTP mechanics are already covered by
// internal/backend/github's httptest suite.
type fakeBackend struct {
	published []string
}

func (f *fakeBackend) UpsertDraft(context.Context, backend.UpsertDraftRequest) error { return nil }
func (f *fakeBackend) Publish(_ context.Context, tag string) error {
	f.published = append(f.published, tag)
	return nil
}

func (f *fakeBackend) CommitURL(string) string          { return "" }
func (f *fakeBackend) CompareURL(string, string) string { return "" }

func (f *fakeBackend) ResolveAuthor(context.Context, string) (backend.AuthorReference, bool, error) {
	return backend.AuthorReference{}, false, nil
}

func (f *fakeBackend) ResolvePR(context.Context, string) (commit.PRReference, bool, error) {
	return commit.PRReference{}, false, nil
}

func multiPlan() *engine.Plan {
	return &engine.Plan{
		Packages: []engine.PackagePlan{
			{Name: "api", SuggestedVersion: "1.0.1"},
			{Name: "web", SuggestedVersion: "1.1.0"},
		},
	}
}

func multiFormat(t *testing.T) *version.Format {
	t.Helper()
	f, err := version.ParseFormat("{{package}}-v{{version}}")
	require.NoError(t, err)
	return f
}

func TestRunMulti_PublishesOnePerPendingPackage(t *testing.T) {
	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runMulti(context.Background(), fb, multiPlan(), multiFormat(t), "", "", &out)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"api-v1.0.1", "web-v1.1.0"}, fb.published)
}

func TestRunMulti_PackageFlagScopesToOnePackage(t *testing.T) {
	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runMulti(context.Background(), fb, multiPlan(), multiFormat(t), "web", "", &out)
	require.NoError(t, err)

	require.Len(t, fb.published, 1, "--package=web must not touch api")
	assert.Equal(t, "web-v1.1.0", fb.published[0])
}

func TestRunMulti_VersionOverrideRequiresPackageFlag(t *testing.T) {
	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runMulti(context.Background(), fb, multiPlan(), multiFormat(t), "", "9.9.9", &out)
	require.Error(t, err)
	assert.Empty(t, fb.published)
}

func TestRunMulti_VersionOverrideAppliesOnlyToScopedPackage(t *testing.T) {
	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runMulti(context.Background(), fb, multiPlan(), multiFormat(t), "api", "9.9.9", &out)
	require.NoError(t, err)

	require.Len(t, fb.published, 1)
	assert.Equal(t, "api-v9.9.9", fb.published[0])
}

func TestRunMulti_VersionOverrideAllowsPackageWithNoPendingChanges(t *testing.T) {
	fb := &fakeBackend{}
	var out bytes.Buffer

	// "docs" has no PackagePlan at all (no pending Entries) — an explicit
	// --version must still let it be (re-)published.
	err := runMulti(context.Background(), fb, multiPlan(), multiFormat(t), "docs", "2.0.0", &out)
	require.NoError(t, err)

	require.Len(t, fb.published, 1)
	assert.Equal(t, "docs-v2.0.0", fb.published[0])
}

func TestRunMulti_UnknownPackageNoOverrideIsError(t *testing.T) {
	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runMulti(context.Background(), fb, multiPlan(), multiFormat(t), "docs", "", &out)
	require.Error(t, err)
	assert.Empty(t, fb.published)
}

func TestRunMulti_NoPendingPackagesIsNotAnError(t *testing.T) {
	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runMulti(context.Background(), fb, &engine.Plan{}, multiFormat(t), "", "", &out)
	require.NoError(t, err)
	assert.Empty(t, fb.published)
	assert.Contains(t, out.String(), "nothing to publish")
}
