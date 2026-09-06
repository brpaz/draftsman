// Package versionfile locates and rewrites the version string inside a
// package's manifest file, for release-strategy: pr (see
// .scratch/pr-release-strategy/spec.md). Detection is auto by marker file,
// with an explicit per-Package override; rewriting is a targeted
// find-and-replace on the version value rather than a full parse/re-marshal,
// so the rest of the file's formatting (key order, indentation, comments)
// is left untouched.
package versionfile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/brpaz/draftsman/internal/config"
)

// Kind is the file format a Target's version string is encoded in.
type Kind string

const (
	KindJSON  Kind = "json"
	KindTOML  Kind = "toml"
	KindPlain Kind = "plain"
)

// Target is one package's detected (or overridden) version file.
type Target struct {
	// Path is relative to the repository root.
	Path string
	Kind Kind
	// Tables are the TOML table names version may live under, tried in
	// order (e.g. ["package"] for Cargo.toml, ["project", "tool.poetry"]
	// for pyproject.toml). Unused for non-TOML Kinds.
	Tables []string
}

// candidate is one built-in auto-detected manifest.
type candidate struct {
	file   string
	kind   Kind
	tables []string
}

// candidates is tried in order — first match in the package directory wins.
var candidates = []candidate{
	{file: "package.json", kind: KindJSON},
	{file: "Cargo.toml", kind: KindTOML, tables: []string{"package"}},
	{file: "pyproject.toml", kind: KindTOML, tables: []string{"project", "tool.poetry"}},
	{file: "composer.json", kind: KindJSON},
}

const (
	goModMarker  = "go.mod"
	fallbackFile = "VERSION"
)

// Detect resolves pkg's version file within repoPath. pkg.VersionFile, if
// set, wins outright with its Kind inferred from its extension. Otherwise
// the first candidate manifest found in the package's directory wins. A
// go.mod present with no override means "no version file for this
// package" (ok=false, err=nil) — Go modules carry no in-repo version
// field, SemVer lives entirely in git tags. Anything else falls back to a
// plain-text VERSION file (which may not exist yet — callers create it).
func Detect(repoPath string, pkg config.Package) (Target, bool, error) {
	dir := filepath.Join(repoPath, pkg.Path)

	if pkg.VersionFile != "" {
		return Target{Path: filepath.Join(pkg.Path, pkg.VersionFile), Kind: kindForExt(pkg.VersionFile)}, true, nil
	}

	for _, c := range candidates {
		exists, err := fileExists(filepath.Join(dir, c.file))
		if err != nil {
			return Target{}, false, err
		}
		if exists {
			return Target{Path: filepath.Join(pkg.Path, c.file), Kind: c.kind, Tables: c.tables}, true, nil
		}
	}

	exists, err := fileExists(filepath.Join(dir, goModMarker))
	if err != nil {
		return Target{}, false, err
	}
	if exists {
		return Target{}, false, nil
	}

	return Target{Path: filepath.Join(pkg.Path, fallbackFile), Kind: KindPlain}, true, nil
}

func kindForExt(path string) Kind {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return KindJSON
	case ".toml":
		return KindTOML
	default:
		return KindPlain
	}
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

var jsonVersionPattern = regexp.MustCompile(`("version"\s*:\s*")([^"]*)(")`)

// tableHeaderPattern matches any TOML table header line, used to find
// where a scoped table's region ends.
var tableHeaderPattern = regexp.MustCompile(`(?m)^\s*\[[^\]]+\]\s*$`)

var tomlVersionPattern = regexp.MustCompile(`(?m)^(\s*version\s*=\s*")([^"]*)(")\s*$`)

// Bump rewrites content to newVersion according to target.Kind, returning
// the new content and the version string it replaced ("" when content is
// empty, e.g. a VERSION file being created for the first time). content is
// nil/empty only valid for KindPlain — JSON/TOML targets are only ever
// detected because the file already exists.
func Bump(target Target, content []byte, newVersion string) (newContent []byte, oldVersion string, err error) {
	if target.Kind == KindPlain {
		return []byte(newVersion + "\n"), strings.TrimSpace(string(content)), nil
	}

	start, end, old, err := locateVersion(target, content)
	if err != nil {
		return nil, "", err
	}

	var buf bytes.Buffer
	buf.Write(content[:start])
	buf.WriteString(newVersion)
	buf.Write(content[end:])
	return buf.Bytes(), old, nil
}

// Read extracts the current version string from content according to
// target.Kind, without modifying it — the read-back counterpart to Bump,
// used at publish time once a release-strategy: pr branch has been merged.
func Read(target Target, content []byte) (string, error) {
	if target.Kind == KindPlain {
		return strings.TrimSpace(string(content)), nil
	}

	_, _, version, err := locateVersion(target, content)
	return version, err
}

// locateVersion finds the version value inside content for target's Kind
// (KindJSON or KindTOML — KindPlain has no such value to locate, callers
// handle it separately), returning its byte range and current value.
func locateVersion(target Target, content []byte) (start, end int, version string, err error) {
	switch target.Kind {
	case KindJSON:
		loc := jsonVersionPattern.FindSubmatchIndex(content)
		if loc == nil {
			return 0, 0, "", fmt.Errorf("versionfile: no \"version\" key found")
		}
		return loc[4], loc[5], string(content[loc[4]:loc[5]]), nil
	case KindTOML:
		region, offset, scoped := findTable(content, target.Tables)
		loc := tomlVersionPattern.FindSubmatchIndex(region)
		if loc == nil {
			hint := ""
			if scoped {
				hint = fmt.Sprintf(" in table(s) %v", target.Tables)
			}
			return 0, 0, "", fmt.Errorf("versionfile: no version key found%s", hint)
		}
		return offset + loc[4], offset + loc[5], string(region[loc[4]:loc[5]]), nil
	default:
		return 0, 0, "", fmt.Errorf("versionfile: unknown kind %q", target.Kind)
	}
}

// findTable returns the byte region of content scoped to the first
// matching table in tables (tried in order): everything after that
// table's header line up to (but not including) the next table header, or
// EOF. scoped is false — and region is the whole of content — when none of
// tables is found, so callers still get a best-effort whole-file search.
func findTable(content []byte, tables []string) (region []byte, offset int, scoped bool) {
	for _, t := range tables {
		headerRe := regexp.MustCompile(`(?m)^\s*\[` + regexp.QuoteMeta(t) + `\]\s*$`)
		loc := headerRe.FindIndex(content)
		if loc == nil {
			continue
		}

		start := loc[1]
		if start < len(content) && content[start] == '\n' {
			start++
		}

		end := len(content)
		if next := tableHeaderPattern.FindIndex(content[start:]); next != nil {
			end = start + next[0]
		}

		return content[start:end], start, true
	}

	return content, 0, false
}
