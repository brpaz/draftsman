package version_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/version"
)

func TestParseAndApply(t *testing.T) {
	v, err := version.Parse("1.2.3")
	require.NoError(t, err)
	assert.Equal(t, version.SemVer{Major: 1, Minor: 2, Patch: 3}, v)
	assert.Equal(t, "1.2.3", v.String())

	assert.Equal(t, "2.0.0", v.Apply(version.BumpMajor).String())
	assert.Equal(t, "1.3.0", v.Apply(version.BumpMinor).String())
	assert.Equal(t, "1.2.4", v.Apply(version.BumpPatch).String())
	assert.Equal(t, "1.2.3", v.Apply(version.BumpNone).String())
}

func TestParse_Invalid(t *testing.T) {
	_, err := version.Parse("not-a-version")
	require.Error(t, err)
}

func TestGreaterThan(t *testing.T) {
	assert.True(t, version.SemVer{Major: 2}.GreaterThan(version.SemVer{Major: 1, Minor: 9, Patch: 9}))
	assert.True(t, version.SemVer{Major: 1, Minor: 1}.GreaterThan(version.SemVer{Major: 1}))
	assert.True(t, version.SemVer{Major: 1, Patch: 1}.GreaterThan(version.SemVer{Major: 1}))
	assert.False(t, version.SemVer{Major: 1}.GreaterThan(version.SemVer{Major: 1}))
}

func TestMax(t *testing.T) {
	assert.Equal(t, version.BumpMajor, version.Max(version.BumpMajor, version.BumpPatch))
	assert.Equal(t, version.BumpMinor, version.Max(version.BumpNone, version.BumpMinor))
}

func TestParseFormat_RequiresVersionPlaceholder(t *testing.T) {
	_, err := version.ParseFormat("v-only-literal")
	require.Error(t, err)
}

func TestFormat_Match(t *testing.T) {
	f, err := version.ParseFormat("v{{version}}")
	require.NoError(t, err)

	v, ok := f.Match("v1.2.3")
	require.True(t, ok)
	assert.Equal(t, "1.2.3", v.String())

	_, ok = f.Match("api-v1.2.3")
	assert.False(t, ok, "doesn't match a different literal prefix")

	_, ok = f.Match("v1.2")
	assert.False(t, ok, "requires a full major.minor.patch")
}

func TestFormat_MatchWithPackagePlaceholder(t *testing.T) {
	f, err := version.ParseFormat("{{package}}-v{{version}}")
	require.NoError(t, err)

	v, ok := f.Match("api-v2.0.0")
	require.True(t, ok)
	assert.Equal(t, "2.0.0", v.String())

	_, ok = f.Match("v2.0.0")
	assert.False(t, ok, "package segment is required by this format")
}
