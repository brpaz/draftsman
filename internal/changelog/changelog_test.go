package changelog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/changelog"
)

func TestPrepend_NewFile(t *testing.T) {
	got := changelog.Prepend(nil, "1.0.0", "2026-09-06", "- did a thing\n")

	want := "# CHANGELOG\n\n## 1.0.0 - 2026-09-06\n\n- did a thing\n"
	assert.Equal(t, want, string(got))
}

func TestPrepend_ExistingFileInsertsBelowHeading(t *testing.T) {
	existing := "# CHANGELOG\n\n## 0.9.0 - 2026-08-01\n\n- older entry\n"

	got := changelog.Prepend([]byte(existing), "1.0.0", "2026-09-06", "- newer entry\n")

	want := "# CHANGELOG\n\n## 1.0.0 - 2026-09-06\n\n- newer entry\n\n## 0.9.0 - 2026-08-01\n\n- older entry\n"
	assert.Equal(t, want, string(got))
}

func TestTopEntry_SingleEntry(t *testing.T) {
	content := changelog.Prepend(nil, "1.0.0", "2026-09-06", "- did a thing\n")

	version, body, ok := changelog.TopEntry(content)
	require.True(t, ok)
	assert.Equal(t, "1.0.0", version)
	assert.Equal(t, "- did a thing", body)
}

func TestTopEntry_StopsBeforeNextEntry(t *testing.T) {
	content := changelog.Prepend(nil, "0.9.0", "2026-08-01", "- older entry\n")
	content = changelog.Prepend(content, "1.0.0", "2026-09-06", "- newer entry\n")

	version, body, ok := changelog.TopEntry(content)
	require.True(t, ok)
	assert.Equal(t, "1.0.0", version)
	assert.Equal(t, "- newer entry", body)
	assert.NotContains(t, body, "older entry")
}

func TestTopEntry_NoEntriesReturnsNotOK(t *testing.T) {
	_, _, ok := changelog.TopEntry([]byte("# CHANGELOG\n\n"))
	assert.False(t, ok)
}

func TestTopEntry_EmptyContentReturnsNotOK(t *testing.T) {
	_, _, ok := changelog.TopEntry(nil)
	assert.False(t, ok)
}

func TestPrepend_NoHeadingAddsOne(t *testing.T) {
	existing := "## 0.9.0 - 2026-08-01\n\n- older entry\n"

	got := changelog.Prepend([]byte(existing), "1.0.0", "2026-09-06", "- newer entry\n")

	want := "# CHANGELOG\n\n## 1.0.0 - 2026-09-06\n\n- newer entry\n\n## 0.9.0 - 2026-08-01\n\n- older entry\n"
	assert.Equal(t, want, string(got))
}
