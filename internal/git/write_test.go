package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/git"
)

func TestPushBranch_CreatesNewBranchWithChangesOnRemote(t *testing.T) {
	local, remote := newRepoWithRemote(t)

	sha, err := git.PushBranch(context.Background(), local, "origin", "release/foo", "HEAD", []git.FileChange{
		{Path: "VERSION", Content: []byte("1.0.0\n")},
	}, "chore(release): 1.0.0")
	require.NoError(t, err)
	assert.NotEmpty(t, sha)

	assert.Equal(t, "1.0.0\n", showRemoteFile(t, remote, "release/foo", "VERSION"))
	assert.Equal(t, git.BotName, remoteLogField(t, remote, "release/foo", "%an"))
	assert.Equal(t, git.BotEmail, remoteLogField(t, remote, "release/foo", "%ae"))
	assert.Equal(t, "chore(release): 1.0.0", remoteLogField(t, remote, "release/foo", "%s"))

	// The local working tree/checkout must be completely untouched.
	_, err = os.Stat(filepath.Join(local, "VERSION"))
	assert.True(t, os.IsNotExist(err), "PushBranch must not write into the local working tree")
	assert.Equal(t, "main", currentBranch(t, local))
}

func TestPushBranch_PreservesOtherFilesFromBaseTree(t *testing.T) {
	local, remote := newRepoWithRemote(t)

	_, err := git.PushBranch(context.Background(), local, "origin", "release/foo", "HEAD", []git.FileChange{
		{Path: "VERSION", Content: []byte("1.0.0\n")},
	}, "chore(release): 1.0.0")
	require.NoError(t, err)

	assert.Equal(t, "hello\n", showRemoteFile(t, remote, "release/foo", "README.md"), "the base tree's existing file must survive untouched")
}

func TestPushBranch_RerunForceUpdatesInPlace(t *testing.T) {
	local, remote := newRepoWithRemote(t)

	first, err := git.PushBranch(context.Background(), local, "origin", "release/foo", "HEAD", []git.FileChange{
		{Path: "VERSION", Content: []byte("1.0.0\n")},
	}, "chore(release): 1.0.0")
	require.NoError(t, err)

	// Simulate more commits landing on default before the PR is merged.
	runOK(t, local, "commit", "--allow-empty", "-m", "feat: something else")

	second, err := git.PushBranch(context.Background(), local, "origin", "release/foo", "HEAD", []git.FileChange{
		{Path: "VERSION", Content: []byte("1.1.0\n")},
	}, "chore(release): 1.1.0")
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
	assert.Equal(t, "1.1.0\n", showRemoteFile(t, remote, "release/foo", "VERSION"))

	parent := remoteLogField(t, remote, "release/foo", "%P")
	base := runOK(t, local, "rev-parse", "HEAD")
	assert.Equal(t, base, parent, "each run rebases from the current default branch tip rather than extending the branch's own history")
}

// newRepoWithRemote sets up a local repo with one commit on "main" plus a
// bare repo added as its "origin" remote, returning both paths.
func newRepoWithRemote(t *testing.T) (local, remote string) {
	t.Helper()

	local = t.TempDir()
	runOK(t, local, "init", "-q", "-b", "main")
	runOK(t, local, "config", "user.email", "test@example.com")
	runOK(t, local, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(local, "README.md"), []byte("hello\n"), 0o644))
	runOK(t, local, "add", "README.md")
	runOK(t, local, "commit", "-q", "-m", "chore: init")

	remote = t.TempDir()
	runOK(t, remote, "init", "-q", "--bare")
	runOK(t, local, "remote", "add", "origin", remote)

	return local, remote
}

func runOK(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), out)
	return strings.TrimSpace(string(out))
}

func showRemoteFile(t *testing.T, remote, ref, path string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", remote, "show", ref+":"+path).CombinedOutput()
	require.NoError(t, err, "git show %s:%s: %s", ref, path, out)
	return string(out)
}

func remoteLogField(t *testing.T, remote, ref, format string) string {
	t.Helper()
	return runOK(t, remote, "log", "-1", "--format="+format, ref)
}

func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	return runOK(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
}
