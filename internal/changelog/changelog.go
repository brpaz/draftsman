// Package changelog maintains the CHANGELOG.md file release-strategy: pr
// commits alongside a version bump — a deliberate, scoped exception to
// draftsman's "never write a changelog file" rule, which continues to hold
// for release-strategy: draft (see .scratch/pr-release-strategy/spec.md).
package changelog

import (
	"fmt"
	"strings"
)

// Heading is the top-level heading a managed CHANGELOG.md starts with.
const Heading = "# CHANGELOG"

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
