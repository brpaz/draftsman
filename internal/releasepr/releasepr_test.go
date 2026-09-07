package releasepr_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/config"
	"github.com/brpaz/draftsman/internal/engine"
	"github.com/brpaz/draftsman/internal/releasepr"
)

func TestCompute_SingleMode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name": "foo", "version": "1.0.0"}`)

	cfg := config.Default()
	plan := &engine.Plan{
		SuggestedVersion: "1.1.0",
		Packages: []engine.PackagePlan{
			{Sections: []engine.Section{{Name: "Features", Entries: []engine.Entry{{Description: "add thing", ShortSHA: "abc1234"}}}}},
		},
	}

	plans, err := releasepr.Compute(dir, cfg, plan, "2026-09-06")
	require.NoError(t, err)
	require.Len(t, plans, 1)

	fp := plans[0]
	assert.False(t, fp.VersionFileNoOp)
	assert.Equal(t, "package.json", fp.VersionFilePath)
	assert.Equal(t, "1.0.0", fp.OldVersion)
	assert.Equal(t, "1.1.0", fp.NewVersion)
	assert.Equal(t, "CHANGELOG.md", fp.ChangelogPath)
	assert.Contains(t, fp.ChangelogEntry, "## 1.1.0 - 2026-09-06")
	assert.Contains(t, fp.ChangelogEntry, "add thing")
}

func TestCompute_SingleMode_NoPendingReleaseReturnsNil(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	plan := &engine.Plan{}

	plans, err := releasepr.Compute(dir, cfg, plan, "2026-09-06")
	require.NoError(t, err)
	assert.Empty(t, plans)
}

func TestCompute_MultiMode_OnePerPendingPackage(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "worker"), 0o755))
	writeFile(t, filepath.Join(dir, "api", "package.json"), `{"version": "2.0.0"}`)
	writeFile(t, filepath.Join(dir, "worker", "go.mod"), "module example.com/worker\n\ngo 1.23\n")

	cfg := config.Default()
	cfg.Mode = config.ModeMulti
	cfg.Packages = []config.Package{
		{Path: "api", Name: "api"},
		{Path: "worker", Name: "worker"},
	}
	plan := &engine.Plan{
		Packages: []engine.PackagePlan{
			{Name: "api", SuggestedVersion: "2.1.0", Sections: []engine.Section{{Name: "Features", Entries: []engine.Entry{{Description: "api change"}}}}},
			{Name: "worker", SuggestedVersion: "1.0.1", Sections: []engine.Section{{Name: "Bug Fixes", Entries: []engine.Entry{{Description: "worker fix"}}}}},
			{Name: "docs"}, // no SuggestedVersion: nothing pending, must be skipped
		},
	}

	plans, err := releasepr.Compute(dir, cfg, plan, "2026-09-06")
	require.NoError(t, err)
	require.Len(t, plans, 2)

	byPkg := map[string]releasepr.FilePlan{}
	for _, fp := range plans {
		byPkg[fp.Package] = fp
	}

	api := byPkg["api"]
	assert.Equal(t, filepath.Join("api", "package.json"), api.VersionFilePath)
	assert.Equal(t, "2.0.0", api.OldVersion)
	assert.Equal(t, filepath.Join("api", "CHANGELOG.md"), api.ChangelogPath)

	worker := byPkg["worker"]
	assert.True(t, worker.VersionFileNoOp, "a Go module package must not get a version-file bump")
	assert.Equal(t, filepath.Join("worker", "CHANGELOG.md"), worker.ChangelogPath)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
