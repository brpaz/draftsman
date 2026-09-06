// Package changelog maintains the CHANGELOG.md file release-strategy: pr
// commits alongside a version bump — a deliberate, scoped exception to
// draftsman's "never write a changelog file" rule, which continues to hold
// for release-strategy: draft (see .scratch/pr-release-strategy/spec.md).
package changelog

import (
	"fmt"
	"regexp"
	"strings"
)

// Heading is the top-level heading a managed CHANGELOG.md starts with.
const Heading = "# CHANGELOG"

// entryHeadingPattern matches a version entry heading written by Prepend:
// "## <version> - <date>". The version is the first whitespace-run, same
// as any SemVer or tag string — never containing a space itself.
var entryHeadingPattern = regexp.MustCompile(`(?m)^## (\S+) - .*$`)

// Prepend returns existing (a CHANGELOG.md's current content, or nil/empty
// if the file doesn't exist yet) with a new entry for version inserted
// immediately below the top-level Heading (added if missing), so the
// newest release always reads first.
func Prepend(existing []byte, version, date, body string) []byte {
	entry := fmt.Sprintf("## %s - %s\n\n%s\n", version, date, strings.TrimRight(body, "\n"))

	text := string(existing)
	if strings.TrimSpace(text) == "" {
		return []byte(Heading + "\n\n" + entry)
	}

	if rest, ok := strings.CutPrefix(text, Heading); ok {
		rest = strings.TrimLeft(rest, "\n")
		return []byte(Heading + "\n\n" + entry + "\n" + rest)
	}

	return []byte(Heading + "\n\n" + entry + "\n" + text)
}

// TopEntry extracts the newest entry from content (a CHANGELOG.md
// maintained by Prepend): the version from its "## <version> - <date>"
// heading, and the body text up to (but not including) the next such
// heading or EOF. ok is false when content has no entry heading at all
// (e.g. an empty or freshly-created file) — used at publish time to read
// back exactly what a merged release-strategy: pr branch's PR carried, see
// .scratch/pr-release-strategy/spec.md.
func TopEntry(content []byte) (version, body string, ok bool) {
	locs := entryHeadingPattern.FindAllSubmatchIndex(content, 2)
	if len(locs) == 0 {
		return "", "", false
	}

	first := locs[0]
	version = string(content[first[2]:first[3]])

	bodyStart := first[1]
	bodyEnd := len(content)
	if len(locs) > 1 {
		bodyEnd = locs[1][0]
	}

	body = strings.TrimSpace(string(content[bodyStart:bodyEnd]))
	return version, body, true
}
