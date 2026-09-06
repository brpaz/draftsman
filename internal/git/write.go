package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BotName/BotEmail identify commits draftsman itself creates on a release
// branch (release-strategy: pr) — used both as the commit author here and,
// later, to detect whether a human has since pushed a manual edit to that
// branch (a HEAD authored by anyone else means "back off", see ticket 04).
const (
	BotName  = "draftsman-bot"
	BotEmail = "draftsman-bot@users.noreply.github.com"
)

// FileChange is one file's new content for PushBranch to write.
type FileChange struct {
	// Path is relative to the repository root.
	Path    string
	Content []byte
}

// PushBranch creates a new commit — baseRef's tree with changes applied,
// authored as BotName/BotEmail — and force-pushes it directly to
// remote as branch, without touching repoPath's working tree or current
// checkout. Built entirely via git plumbing (hash-object, a scratch index,
// write-tree, commit-tree): every run starts fresh from baseRef rather than
// extending the branch's previous history, so the push is always
// non-fast-forward and must be forced — the same "rebase from default each
// time" shape as a release-please release PR. Returns the new commit SHA.
//
// remote is a git remote name (or URL) already configured with whatever
// credentials the push needs (e.g. a token embedded in an https:// remote
// URL) — this function is push-mechanics only and carries no auth concept
// of its own, same as every other command in this package shelling out to
// the local git binary.
func PushBranch(ctx context.Context, repoPath, remote, branch, baseRef string, changes []FileChange, message string) (string, error) {
	baseSHA, err := revParse(ctx, repoPath, baseRef)
	if err != nil {
		return "", fmt.Errorf("resolving base ref %q: %w", baseRef, err)
	}
	baseTree, err := revParse(ctx, repoPath, baseRef+"^{tree}")
	if err != nil {
		return "", fmt.Errorf("resolving base tree for %q: %w", baseRef, err)
	}

	indexFile, err := os.CreateTemp("", "draftsman-index-*")
	if err != nil {
		return "", fmt.Errorf("creating scratch index: %w", err)
	}
	indexPath := indexFile.Name()
	_ = indexFile.Close()
	defer func() { _ = os.Remove(indexPath) }()
	indexEnv := []string{"GIT_INDEX_FILE=" + indexPath}

	if _, err := runGit(ctx, repoPath, indexEnv, "read-tree", baseTree); err != nil {
		return "", fmt.Errorf("reading base tree into scratch index: %w", err)
	}

	for _, change := range changes {
		blobSHA, err := hashObject(ctx, repoPath, change.Content)
		if err != nil {
			return "", fmt.Errorf("hashing %s: %w", change.Path, err)
		}
		cacheInfo := fmt.Sprintf("100644,%s,%s", blobSHA, change.Path)
		if _, err := runGit(ctx, repoPath, indexEnv, "update-index", "--add", "--cacheinfo", cacheInfo); err != nil {
			return "", fmt.Errorf("staging %s: %w", change.Path, err)
		}
	}

	newTree, err := runGit(ctx, repoPath, indexEnv, "write-tree")
	if err != nil {
		return "", fmt.Errorf("writing tree: %w", err)
	}
	newTree = strings.TrimSpace(newTree)

	commitEnv := []string{
		"GIT_AUTHOR_NAME=" + BotName, "GIT_AUTHOR_EMAIL=" + BotEmail,
		"GIT_COMMITTER_NAME=" + BotName, "GIT_COMMITTER_EMAIL=" + BotEmail,
	}
	newCommit, err := runGit(ctx, repoPath, commitEnv, "commit-tree", newTree, "-p", baseSHA, "-m", message)
	if err != nil {
		return "", fmt.Errorf("creating commit: %w", err)
	}
	newCommit = strings.TrimSpace(newCommit)

	refspec := fmt.Sprintf("+%s:refs/heads/%s", newCommit, branch)
	if _, err := runGit(ctx, repoPath, nil, "push", remote, refspec); err != nil {
		return "", fmt.Errorf("pushing %s to %s: %w", branch, remote, err)
	}

	return newCommit, nil
}

func revParse(ctx context.Context, repoPath, ref string) (string, error) {
	out, err := runGit(ctx, repoPath, nil, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func hashObject(ctx context.Context, repoPath string, content []byte) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "hash-object", "-w", "--stdin")
	cmd.Stdin = bytes.NewReader(content)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git hash-object: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func runGit(ctx context.Context, repoPath string, extraEnv []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoPath}, args...)...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
