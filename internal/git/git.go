// Package git reads commit history from a local repository.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Commit is a single commit as read from git log, oldest fields only —
// callers parse Message themselves (see internal/commit).
type Commit struct {
	SHA     string
	Message string
}

const fieldSep = "\x1f"

// Log returns every commit reachable from HEAD in repoPath, most recent
// first. If since is non-empty, only commits after that ref are returned
// (since..HEAD) — used to bound history to "since the last release". A
// repository with no commits yet returns an empty slice, not an error.
func Log(ctx context.Context, repoPath, since string) ([]Commit, error) {
	if err := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--verify", "-q", "HEAD").Run(); err != nil {
		return nil, nil
	}

	rangeArg := "HEAD"
	if since != "" {
		rangeArg = since + "..HEAD"
	}

	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "log", "-z", "--pretty=format:%H"+fieldSep+"%B", rangeArg)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git log: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	raw := stdout.String()
	if raw == "" {
		return nil, nil
	}

	records := strings.Split(raw, "\x00")
	commits := make([]Commit, 0, len(records))
	for _, rec := range records {
		if rec == "" {
			continue
		}
		sha, message, found := strings.Cut(rec, fieldSep)
		if !found {
			continue
		}
		commits = append(commits, Commit{
			SHA:     sha,
			Message: strings.TrimSuffix(message, "\n"),
		})
	}
	return commits, nil
}

// ChangedFiles returns the paths changed by sha, relative to the repo root.
// --root makes this correct for a commit with no parent (diffed against an
// empty tree) as well as any ordinary commit.
func ChangedFiles(ctx context.Context, repoPath, sha string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "diff-tree",
		"--no-commit-id", "--name-only", "-r", "--root", sha)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff-tree %s: %w: %s", sha, err, strings.TrimSpace(stderr.String()))
	}

	raw := strings.TrimSpace(stdout.String())
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}

// Tags returns every tag reachable from HEAD in repoPath. A repository with
// no commits yet returns an empty slice, not an error.
func Tags(ctx context.Context, repoPath string) ([]string, error) {
	if err := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--verify", "-q", "HEAD").Run(); err != nil {
		return nil, nil
	}

	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "tag", "--merged", "HEAD")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git tag --merged: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	raw := strings.TrimSpace(stdout.String())
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}
