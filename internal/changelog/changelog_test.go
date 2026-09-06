package changelog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

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

func TestPrepend_NoHeadingAddsOne(t *testing.T) {
	existing := "## 0.9.0 - 2026-08-01\n\n- older entry\n"

	got := changelog.Prepend([]byte(existing), "1.0.0", "2026-09-06", "- newer entry\n")

	want := "# CHANGELOG\n\n## 1.0.0 - 2026-09-06\n\n- newer entry\n\n## 0.9.0 - 2026-08-01\n\n- older entry\n"
	assert.Equal(t, want, string(got))
}
