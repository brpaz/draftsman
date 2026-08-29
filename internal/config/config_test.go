package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/config"
)

func TestLoad_MissingDefaultPathReturnsDefaults(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), ".draftsman.yml"), false)
	require.NoError(t, err)
	assert.Equal(t, config.Default(), cfg)
}

func TestLoad_MissingExplicitPathErrors(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), ".draftsman.yml"), true)
	require.Error(t, err)
}

func TestLoad_OverridesLayerOnDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".draftsman.yml")
	writeFile(t, path, `
mode: multi
skip-changelog-trailer: Ignore-Me
categories:
  - type: fix
    section: Bug Fixes
  - type: feat
    section: New Stuff
  - type: docs
    section: Documentation
`)

	cfg, err := config.Load(path, true)
	require.NoError(t, err)

	assert.Equal(t, config.ModeMulti, cfg.Mode)
	assert.Equal(t, "Ignore-Me", cfg.SkipChangelogTrailer)
	// Full replace, in the order given — reorder (fix before feat) and add
	// (docs) both take effect, not just remap.
	assert.Equal(t, []config.Category{
		{Type: "fix", Section: "Bug Fixes"},
		{Type: "feat", Section: "New Stuff"},
		{Type: "docs", Section: "Documentation"},
	}, cfg.Categories)
	// Untouched field keeps the default.
	assert.Equal(t, config.Default().Template, cfg.Template)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
