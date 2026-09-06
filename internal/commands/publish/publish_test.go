package publish

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/commit"
	"github.com/brpaz/draftsman/internal/config"
	"github.com/brpaz/draftsman/internal/engine"
	"github.com/brpaz/draftsman/internal/version"
)

// fakeBackend is a minimal backend.Backend double recording Publish calls,
// so multi-package wiring can be verified without a real HTTP server — the
// GitHub adapter's own HTTP mechanics are already covered by
// internal/backend/github's httptest suite.
type fakeBackend struct {
	published       []string
	createdReleases []createdRelease
}

type createdRelease struct {
	tag, name, body string
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

func (f *fakeBackend) UpsertReleasePR(context.Context, backend.UpsertReleasePRRequest) (backend.ReleasePR, error) {
	return backend.ReleasePR{}, nil
}

func (f *fakeBackend) GitRemoteURL() string { return "" }

func (f *fakeBackend) CreateRelease(_ context.Context, tag, name, body string) error {
	f.createdReleases = append(f.createdReleases, createdRelease{tag: tag, name: name, body: body})
	return nil
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

func TestRunPR_SingleMode_PublishesFromMergedChangelogAndVersionFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name": "demo", "version": "1.1.0"}`)
	writeFile(t, filepath.Join(dir, "CHANGELOG.md"), "# CHANGELOG\n\n## 1.1.0 - 2026-09-06\n\n## Features\n- add thing\n")

	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runPR(context.Background(), dir, config.Default(), fb, "", &out)
	require.NoError(t, err)

	require.Len(t, fb.createdReleases, 1)
	assert.Equal(t, "v1.1.0", fb.createdReleases[0].tag)
	assert.Contains(t, fb.createdReleases[0].body, "add thing")
	assert.Contains(t, out.String(), "published v1.1.0")
}

func TestRunPR_GoModuleFallsBackToChangelogHeadingVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/demo\n\ngo 1.23\n")
	writeFile(t, filepath.Join(dir, "CHANGELOG.md"), "# CHANGELOG\n\n## 1.2.0 - 2026-09-06\n\n## Features\n- go thing\n")

	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runPR(context.Background(), dir, config.Default(), fb, "", &out)
	require.NoError(t, err)

	require.Len(t, fb.createdReleases, 1)
	assert.Equal(t, "v1.2.0", fb.createdReleases[0].tag, "no version file for a Go module: fall back to the changelog heading")
}

func TestRunPR_MultiMode_PublishesOnlyPackagesWithMergedEntries(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "web"), 0o755))
	writeFile(t, filepath.Join(dir, "api", "package.json"), `{"version": "2.1.0"}`)
	writeFile(t, filepath.Join(dir, "api", "CHANGELOG.md"), "# CHANGELOG\n\n## 2.1.0 - 2026-09-06\n\n## Features\n- api thing\n")
	// "web" has no CHANGELOG.md yet: nothing merged for it.

	cfg := config.Default()
	cfg.Mode = config.ModeMulti
	cfg.TagFormat = "{{package}}-v{{version}}"
	cfg.Packages = []config.Package{
		{Path: "api", Name: "api"},
		{Path: "web", Name: "web"},
	}

	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runPR(context.Background(), dir, cfg, fb, "", &out)
	require.NoError(t, err)

	require.Len(t, fb.createdReleases, 1)
	assert.Equal(t, "api-v2.1.0", fb.createdReleases[0].tag)
}

func TestRunPR_PackageFlagScopesToOnePackage(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "web"), 0o755))
	writeFile(t, filepath.Join(dir, "api", "package.json"), `{"version": "2.1.0"}`)
	writeFile(t, filepath.Join(dir, "api", "CHANGELOG.md"), "# CHANGELOG\n\n## 2.1.0 - 2026-09-06\n\n- api thing\n")
	writeFile(t, filepath.Join(dir, "web", "package.json"), `{"version": "1.0.0"}`)
	writeFile(t, filepath.Join(dir, "web", "CHANGELOG.md"), "# CHANGELOG\n\n## 1.0.0 - 2026-09-06\n\n- web thing\n")

	cfg := config.Default()
	cfg.Mode = config.ModeMulti
	cfg.Packages = []config.Package{{Path: "api", Name: "api"}, {Path: "web", Name: "web"}}
	cfg.TagFormat = "{{package}}-v{{version}}"

	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runPR(context.Background(), dir, cfg, fb, "web", &out)
	require.NoError(t, err)

	require.Len(t, fb.createdReleases, 1, "--package=web must not touch api")
	assert.Equal(t, "web-v1.0.0", fb.createdReleases[0].tag)
}

func TestRunPR_NoMergedChangelogIsNotAnError(t *testing.T) {
	dir := t.TempDir()

	fb := &fakeBackend{}
	var out bytes.Buffer

	err := runPR(context.Background(), dir, config.Default(), fb, "", &out)
	require.NoError(t, err)
	assert.Empty(t, fb.createdReleases)
	assert.Contains(t, out.String(), "nothing to publish")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
