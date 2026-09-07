package backend_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brpaz/draftsman/internal/backend"
)

func TestFormatGitRemoteURL(t *testing.T) {
	got := backend.FormatGitRemoteURL("https://github.com", "x-access-token", "s3cr3t", "brpaz/draftsman")
	assert.Equal(t, "https://x-access-token:s3cr3t@github.com/brpaz/draftsman.git", got)
}

func TestFormatGitRemoteURL_SelfHostedBaseURL(t *testing.T) {
	got := backend.FormatGitRemoteURL("https://gitea.example.com", "oauth2", "tok", "org/repo")
	assert.Equal(t, "https://oauth2:tok@gitea.example.com/org/repo.git", got)
}
