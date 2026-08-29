package commit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brpaz/draftsman/internal/commit"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    commit.ParsedCommit
		wantOk  bool
	}{
		{
			name:    "simple feat",
			message: "feat: add login page",
			want:    commit.ParsedCommit{Type: "feat", Description: "add login page"},
			wantOk:  true,
		},
		{
			name:    "fix with scope",
			message: "fix(auth): correct token expiry check",
			want:    commit.ParsedCommit{Type: "fix", Scope: "auth", Description: "correct token expiry check"},
			wantOk:  true,
		},
		{
			name:    "breaking change marker",
			message: "feat(api)!: remove deprecated endpoint",
			want:    commit.ParsedCommit{Type: "feat", Scope: "api", Breaking: true, Description: "remove deprecated endpoint"},
			wantOk:  true,
		},
		{
			name:    "type normalized to lowercase",
			message: "Feat: uppercase type",
			want:    commit.ParsedCommit{Type: "feat", Description: "uppercase type"},
			wantOk:  true,
		},
		{
			name:    "only first line matters",
			message: "chore: bump deps\n\nsome body text\n\nFooter: value",
			want:    commit.ParsedCommit{Type: "chore", Description: "bump deps"},
			wantOk:  true,
		},
		{
			name:    "breaking change via footer",
			message: "feat: remove deprecated endpoint\n\nBREAKING CHANGE: the /v1 endpoint is gone",
			want:    commit.ParsedCommit{Type: "feat", Breaking: true, Description: "remove deprecated endpoint"},
			wantOk:  true,
		},
		{
			name:    "breaking change via hyphenated footer",
			message: "feat: remove deprecated endpoint\n\nBREAKING-CHANGE: the /v1 endpoint is gone",
			want:    commit.ParsedCommit{Type: "feat", Breaking: true, Description: "remove deprecated endpoint"},
			wantOk:  true,
		},
		{
			name:    "merge commit is not conventional",
			message: "Merge pull request #42 from brpaz/some-branch",
			wantOk:  false,
		},
		{
			name:    "no colon",
			message: "just a plain message",
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := commit.Parse(tt.message)
			assert.Equal(t, tt.wantOk, ok)
			if tt.wantOk {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTrailers(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    map[string]string
	}{
		{
			name:    "header only, no footers",
			message: "feat: add login page",
			want:    map[string]string{},
		},
		{
			name:    "header with colon is not mistaken for a trailer",
			message: "chore: update README: fix typo",
			want:    map[string]string{},
		},
		{
			name:    "single footer trailer",
			message: "chore: bump lockfile\n\nSkip-Changelog: true",
			want:    map[string]string{"Skip-Changelog": "true"},
		},
		{
			name:    "multiple trailers, body text untouched",
			message: "fix(auth): rotate keys\n\nRotated after incident.\n\nSkip-Changelog: true\nReviewed-on: https://example.com/pulls/9",
			want: map[string]string{
				"Skip-Changelog": "true",
				"Reviewed-on":    "https://example.com/pulls/9",
			},
		},
		{
			name:    "body text that looks like a trailer but isn't at the bottom",
			message: "fix: x\n\nSee: https://example.com\n\nmore body",
			want:    map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, commit.Trailers(tt.message))
		})
	}
}

func TestExtractPRReference(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    commit.PRReference
		wantOk  bool
	}{
		{
			name:    "github squash suffix",
			message: "feat: add login page (#123)",
			want:    commit.PRReference{Number: 123},
			wantOk:  true,
		},
		{
			name:    "gitea/forgejo Reviewed-on trailer",
			message: "fix(auth): rotate keys\n\nRotated after incident.\n\nReviewed-on: https://gitea.example.com/brpaz/draftsman/pulls/9",
			want:    commit.PRReference{Number: 9, Link: "https://gitea.example.com/brpaz/draftsman/pulls/9"},
			wantOk:  true,
		},
		{
			name:    "neither form present",
			message: "chore: bump lockfile",
			wantOk:  false,
		},
		{
			name:    "parenthetical number not at end of header is not a PR ref",
			message: "fix: resolve (#3) partially, more to do",
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := commit.ExtractPRReference(tt.message)
			assert.Equal(t, tt.wantOk, ok)
			if tt.wantOk {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestHasSkipChangelogTag(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{
			name:    "tag in subject line",
			message: "chore: bump lockfile [skip changelog]",
			want:    true,
		},
		{
			name:    "tag is case-insensitive",
			message: "chore: bump lockfile [SKIP CHANGELOG]",
			want:    true,
		},
		{
			name:    "tag in body, not just subject",
			message: "chore: bump lockfile\n\nNot user-facing. [skip changelog]",
			want:    true,
		},
		{
			name:    "no tag",
			message: "feat: add login page",
			want:    false,
		},
		{
			name:    "footer trailer alone does not count as the tag",
			message: "chore: bump lockfile\n\nSkip-Changelog: true",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, commit.HasSkipChangelogTag(tt.message))
		})
	}
}
