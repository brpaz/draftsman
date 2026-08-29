// Package version parses and increments SemVer versions, and matches them
// against a configurable tag-format template.
package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Bump is a SemVer increment severity. Zero value is BumpNone (no bump).
type Bump int

const (
	BumpNone Bump = iota
	BumpPatch
	BumpMinor
	BumpMajor
)

// Max returns the higher-severity of a and b.
func Max(a, b Bump) Bump {
	if b > a {
		return b
	}
	return a
}

// SemVer is a parsed major.minor.patch version. Pre-release and build
// metadata aren't supported — tag-format's {{version}} placeholder only
// matches bare major.minor.patch tags.
type SemVer struct {
	Major, Minor, Patch int
}

func (v SemVer) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// GreaterThan reports whether v is a later version than o.
func (v SemVer) GreaterThan(o SemVer) bool {
	if v.Major != o.Major {
		return v.Major > o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor > o.Minor
	}
	return v.Patch > o.Patch
}

// Apply returns the version after applying bump. BumpNone returns v unchanged.
func (v SemVer) Apply(bump Bump) SemVer {
	switch bump {
	case BumpMajor:
		return SemVer{v.Major + 1, 0, 0}
	case BumpMinor:
		return SemVer{v.Major, v.Minor + 1, 0}
	case BumpPatch:
		return SemVer{v.Major, v.Minor, v.Patch + 1}
	default:
		return v
	}
}

// Parse reads a bare "major.minor.patch" string.
func Parse(s string) (SemVer, error) {
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return SemVer{}, fmt.Errorf("invalid version %q: want major.minor.patch", s)
	}

	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return SemVer{}, fmt.Errorf("invalid version %q: %w", s, err)
		}
		nums[i] = n
	}
	return SemVer{Major: nums[0], Minor: nums[1], Patch: nums[2]}, nil
}

var placeholderRe = regexp.MustCompile(`\{\{(version|package)\}\}`)

// Format is a parsed tag-format template such as "v{{version}}" or
// "{{package}}-v{{version}}". {{package}} is accepted here even though
// single-mode matching (Match) never uses it — multi-mode range-finding
// reuses this same parser once it exists.
type Format struct {
	raw   string
	regex *regexp.Regexp
}

// ParseFormat compiles raw into a matcher. raw must contain {{version}} —
// a tag format that can't identify a version can't be used to find "the
// last release".
func ParseFormat(raw string) (*Format, error) {
	if !strings.Contains(raw, "{{version}}") {
		return nil, fmt.Errorf("tag format %q must contain {{version}}", raw)
	}

	var pattern strings.Builder
	pattern.WriteByte('^')
	last := 0
	for _, loc := range placeholderRe.FindAllStringIndex(raw, -1) {
		pattern.WriteString(regexp.QuoteMeta(raw[last:loc[0]]))
		switch raw[loc[0]:loc[1]] {
		case "{{version}}":
			pattern.WriteString(`(?P<version>\d+\.\d+\.\d+)`)
		case "{{package}}":
			pattern.WriteString(`(?P<package>[^/]+)`)
		}
		last = loc[1]
	}
	pattern.WriteString(regexp.QuoteMeta(raw[last:]))
	pattern.WriteByte('$')

	re, err := regexp.Compile(pattern.String())
	if err != nil {
		return nil, fmt.Errorf("compiling tag format %q: %w", raw, err)
	}

	return &Format{raw: raw, regex: re}, nil
}

// Render substitutes ver (and pkg, for a {{package}} placeholder, if any)
// into the format's template, producing a concrete tag string — the
// counterpart to Match, used when creating a new tag rather than finding an
// existing one.
func (f *Format) Render(ver, pkg string) string {
	s := strings.ReplaceAll(f.raw, "{{version}}", ver)
	s = strings.ReplaceAll(s, "{{package}}", pkg)
	return s
}

// ForPackage returns a Format with {{package}} resolved to name, so Match
// only matches tags belonging to that specific package (multi mode).
func (f *Format) ForPackage(name string) (*Format, error) {
	resolved := strings.ReplaceAll(f.raw, "{{package}}", name)
	return ParseFormat(resolved)
}

// Match reports whether tag matches the format, returning its version when so.
func (f *Format) Match(tag string) (SemVer, bool) {
	m := f.regex.FindStringSubmatch(tag)
	if m == nil {
		return SemVer{}, false
	}

	idx := f.regex.SubexpIndex("version")
	if idx < 0 || idx >= len(m) {
		return SemVer{}, false
	}

	v, err := Parse(m[idx])
	if err != nil {
		return SemVer{}, false
	}
	return v, true
}
