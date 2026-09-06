package versionfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/config"
	"github.com/brpaz/draftsman/internal/versionfile"
)

func TestDetect_PackageJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name": "foo", "version": "1.2.3"}`)

	target, ok, err := versionfile.Detect(dir, config.Package{})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "package.json", target.Path)
	assert.Equal(t, versionfile.KindJSON, target.Kind)
}

func TestDetect_CargoToml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"foo\"\nversion = \"1.2.3\"\n")

	target, ok, err := versionfile.Detect(dir, config.Package{})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "Cargo.toml", target.Path)
	assert.Equal(t, versionfile.KindTOML, target.Kind)
}

func TestDetect_PyprojectToml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), "[project]\nname = \"foo\"\nversion = \"1.2.3\"\n")

	target, ok, err := versionfile.Detect(dir, config.Package{})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "pyproject.toml", target.Path)
}

func TestDetect_ComposerJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "composer.json"), `{"name": "foo/bar", "version": "1.2.3"}`)

	target, ok, err := versionfile.Detect(dir, config.Package{})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "composer.json", target.Path)
	assert.Equal(t, versionfile.KindJSON, target.Kind)
}

func TestDetect_GoModIsNoOp(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.23\n")

	_, ok, err := versionfile.Detect(dir, config.Package{})
	require.NoError(t, err)
	assert.False(t, ok, "a Go module has no in-repo version field to bump")
}

func TestDetect_FallsBackToVersionFile(t *testing.T) {
	dir := t.TempDir()

	target, ok, err := versionfile.Detect(dir, config.Package{})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "VERSION", target.Path)
	assert.Equal(t, versionfile.KindPlain, target.Kind)
}

func TestDetect_PackagePathScopesDetection(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	writeFile(t, filepath.Join(dir, "sub", "package.json"), `{"version": "0.1.0"}`)

	target, ok, err := versionfile.Detect(dir, config.Package{Path: "sub"})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, filepath.Join("sub", "package.json"), target.Path)
}

func TestDetect_ExplicitOverrideWins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"version": "1.0.0"}`)
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[package]\nversion = \"1.0.0\"\n")

	target, ok, err := versionfile.Detect(dir, config.Package{VersionFile: "Cargo.toml"})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "Cargo.toml", target.Path)
	assert.Equal(t, versionfile.KindTOML, target.Kind, "override infers Kind from the extension")
}

func TestBump_JSON(t *testing.T) {
	content := []byte(`{
  "name": "foo",
  "version": "1.2.3",
  "dependencies": {"bar": "^2.0.0"}
}`)

	newContent, old, err := versionfile.Bump(versionfile.Target{Kind: versionfile.KindJSON}, content, "1.3.0")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", old)
	assert.Contains(t, string(newContent), `"version": "1.3.0"`)
	assert.Contains(t, string(newContent), `"bar": "^2.0.0"`, "unrelated keys are untouched")
}

func TestBump_JSON_NoVersionKeyErrors(t *testing.T) {
	_, _, err := versionfile.Bump(versionfile.Target{Kind: versionfile.KindJSON}, []byte(`{"name": "foo"}`), "1.0.0")
	require.Error(t, err)
}

func TestBump_TOML_ScopedToTable(t *testing.T) {
	content := []byte(`[package]
name = "foo"
version = "1.2.3"

[dependencies]
version = "9.9.9"
`)

	target := versionfile.Target{Kind: versionfile.KindTOML, Tables: []string{"package"}}
	newContent, old, err := versionfile.Bump(target, content, "1.3.0")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", old)
	assert.Contains(t, string(newContent), "[package]\nname = \"foo\"\nversion = \"1.3.0\"")
	assert.Contains(t, string(newContent), "[dependencies]\nversion = \"9.9.9\"", "a same-named key in another table must not be touched")
}

func TestBump_TOML_TriesTablesInOrder(t *testing.T) {
	content := []byte("[tool.poetry]\nname = \"foo\"\nversion = \"0.5.0\"\n")

	target := versionfile.Target{Kind: versionfile.KindTOML, Tables: []string{"project", "tool.poetry"}}
	newContent, old, err := versionfile.Bump(target, content, "0.6.0")
	require.NoError(t, err)
	assert.Equal(t, "0.5.0", old)
	assert.Contains(t, string(newContent), "version = \"0.6.0\"")
}

func TestBump_Plain_CreatesNewFile(t *testing.T) {
	newContent, old, err := versionfile.Bump(versionfile.Target{Kind: versionfile.KindPlain}, nil, "1.0.0")
	require.NoError(t, err)
	assert.Empty(t, old)
	assert.Equal(t, "1.0.0\n", string(newContent))
}

func TestBump_Plain_ReplacesExisting(t *testing.T) {
	newContent, old, err := versionfile.Bump(versionfile.Target{Kind: versionfile.KindPlain}, []byte("0.9.0\n"), "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "0.9.0", old)
	assert.Equal(t, "1.0.0\n", string(newContent))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
