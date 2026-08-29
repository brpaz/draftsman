// Package commit parses Conventional Commit messages.
package commit

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	headerRe = regexp.MustCompile(`^([A-Za-z]+)(\(([^)]+)\))?(!)?:\s+(.+)$`)
	// "BREAKING CHANGE" (with a space) is a fixed literal token defined by
	// the Conventional Commits spec, not a general trailer key — every
	// other trailer follows the normal hyphenated-key convention.
	trailerRe = regexp.MustCompile(`^(BREAKING CHANGE|[A-Za-z][A-Za-z0-9-]*): (.*)$`)
)

// ParsedCommit is the structured form of a Conventional Commit header.
type ParsedCommit struct {
	Type        string
	Scope       string
	Breaking    bool
	Description string
}

// Parse reads a commit message's header line as a Conventional Commit. ok is
// false when the header doesn't match the Conventional Commits grammar —
// callers drop such commits rather than guessing at their meaning (ADR-0003).
func Parse(message string) (parsed ParsedCommit, ok bool) {
	header, _, _ := strings.Cut(message, "\n")
	header = strings.TrimSpace(header)

	m := headerRe.FindStringSubmatch(header)
	if m == nil {
		return ParsedCommit{}, false
	}

	breaking := m[4] == "!"
	if !breaking {
		trailers := Trailers(message)
		_, spaced := trailers["BREAKING CHANGE"]
		_, hyphenated := trailers["BREAKING-CHANGE"]
		breaking = spaced || hyphenated
	}

	return ParsedCommit{
		Type:        strings.ToLower(m[1]),
		Scope:       m[3],
		Breaking:    breaking,
		Description: strings.TrimSpace(m[5]),
	}, true
}

// PRReference is a pull/merge request number resolved directly from commit
// message text (see ADR-0001). Link is only populated when the text itself
// carried a full URL (Gitea/Forgejo's "Reviewed-on:" trailer) — GitHub's
// "(#N)" squash suffix carries no URL, so Link stays empty for that form;
// a canonical link for it comes from the live API lookup (ticket 08), not
// from guessing a URL out of thin air here.
type PRReference struct {
	Number int
	Link   string
}

var (
	githubSquashRe = regexp.MustCompile(`\(#(\d+)\)\s*$`)
	reviewedOnRe   = regexp.MustCompile(`^(https?://\S+/pulls/(\d+))$`)
)

// ExtractPRReference looks for a PR reference embedded directly in a
// commit's message text: GitHub squash-merge's "(#N)" title suffix, or
// Gitea/Forgejo squash-merge's "Reviewed-on: .../pulls/N" footer trailer.
// ok is false when neither form is present — callers attach no reference
// rather than guessing one (ADR-0001).
func ExtractPRReference(message string) (PRReference, bool) {
	header, _, _ := strings.Cut(message, "\n")
	if m := githubSquashRe.FindStringSubmatch(strings.TrimSpace(header)); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return PRReference{Number: n}, true
		}
	}

	if reviewedOn, ok := Trailers(message)["Reviewed-on"]; ok {
		if m := reviewedOnRe.FindStringSubmatch(reviewedOn); m != nil {
			if n, err := strconv.Atoi(m[2]); err == nil {
				return PRReference{Number: n, Link: m[1]}, true
			}
		}
	}

	return PRReference{}, false
}

var skipChangelogTagRe = regexp.MustCompile(`(?i)\[skip changelog\]`)

// HasSkipChangelogTag reports whether message contains a "[skip changelog]"
// tag, case-insensitive and anywhere in the message text — the same
// "[skip ci]"-style convention CI systems use, so it works in a single-line
// "git commit -m" without needing a footer trailer. This is a fixed,
// non-configurable literal, unlike cfg.SkipChangelogTrailer's footer key.
func HasSkipChangelogTag(message string) bool {
	return skipChangelogTagRe.MatchString(message)
}

// Trailers returns the git-trailer-style footer of a commit message — the
// contiguous block of "Key: value" lines at the very end, after the last
// blank line. The header line is never treated as a trailer, even for a
// single-line message that happens to contain a colon.
func Trailers(message string) map[string]string {
	lines := strings.Split(strings.TrimRight(message, "\n"), "\n")

	i := len(lines) - 1
	for i > 0 && strings.TrimSpace(lines[i]) == "" {
		i--
	}

	trailers := map[string]string{}
	for i > 0 {
		m := trailerRe.FindStringSubmatch(lines[i])
		if m == nil {
			break
		}
		trailers[m[1]] = m[2]
		i--
	}

	return trailers
}
