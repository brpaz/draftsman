package engine_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brpaz/draftsman/internal/backend"
	"github.com/brpaz/draftsman/internal/commit"
	"github.com/brpaz/draftsman/internal/config"
	"github.com/brpaz/draftsman/internal/engine"
)

// fakeBackend is a minimal backend.Backend double for testing the
// PR-Reference fallback path — UpsertDraft/Publish aren't exercised by
// engine.Compute, only ResolvePR is.
type fakeBackend struct {
	resolvePR func(ctx context.Context, sha string) (commit.PRReference, bool, error)
}

func (f *fakeBackend) UpsertDraft(context.Context, backend.UpsertDraftRequest) error { return nil }
func (f *fakeBackend) Publish(context.Context, string) error                         { return nil }
func (f *fakeBackend) ResolvePR(ctx context.Context, sha string) (commit.PRReference, bool, error) {
	return f.resolvePR(ctx, sha)
}

// runGit runs a git subcommand against dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// newRepo creates an empty, configured real git repository in a temp dir.
// Real git, no mocking — matches the confirmed test seam for engine.Compute.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	return dir
}

// commitMessage writes a unique change to a fixed file and commits msg.
func commitMessage(t *testing.T, dir, msg string, seq int) {
	t.Helper()
	file := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(file, []byte(fmt.Sprintf("commit %d\n", seq)), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", msg)
}

// tagRepo tags dir's current HEAD.
func tagRepo(t *testing.T, dir, name string) {
	t.Helper()
	runGit(t, dir, "tag", name)
}

// initRepo creates a real git repository with one commit per message,
// oldest first.
func initRepo(t *testing.T, messages ...string) string {
	t.Helper()
	dir := newRepo(t)
	for i, msg := range messages {
		commitMessage(t, dir, msg, i)
	}
	return dir
}

// commitAt is one commit to create via initRepoAt: message plus the paths
// (relative to the repo root) it should touch.
type commitAt struct {
	message string
	files   []string
}

// initRepoAt creates a real git repository with one commit per commitAt,
// oldest first, each touching exactly the given file paths — for tests that
// need commits scoped to specific monorepo package directories.
func initRepoAt(t *testing.T, commits ...commitAt) string {
	t.Helper()
	dir := newRepo(t)
	for i, c := range commits {
		commitFiles(t, dir, c.message, i, c.files...)
	}
	return dir
}

// commitFiles writes a unique change to each of files (creating directories
// as needed) and commits msg — for tests that need commits scoped to
// specific paths and interleaved with tags (e.g. multi-mode range-finding).
func commitFiles(t *testing.T, dir, msg string, seq int, files ...string) {
	t.Helper()
	for _, f := range files {
		path := filepath.Join(dir, f)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf("commit %d\n", seq)), 0o644))
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", msg)
}

// soleSections returns the implicit single package's Sections — a
// convenience for tests that don't configure Packages.
func soleSections(t *testing.T, plan *engine.Plan) []engine.Section {
	t.Helper()
	if len(plan.Packages) == 0 {
		return nil
	}
	require.Len(t, plan.Packages, 1)
	require.Empty(t, plan.Packages[0].Name)
	return plan.Packages[0].Sections
}

func TestCompute(t *testing.T) {
	repo := initRepo(t,
		"feat: add login page",
		"fix(auth): correct token expiry check",
		"chore: bump deps",
		"Merge pull request #1 from brpaz/x",
	)

	plan, err := engine.Compute(context.Background(), repo, config.Default(), nil)
	require.NoError(t, err)

	sections := soleSections(t, plan)
	require.Len(t, sections, 3)

	byName := map[string]engine.Section{}
	for _, s := range sections {
		byName[s.Name] = s
	}

	require.Contains(t, byName, "Features")
	require.Len(t, byName["Features"].Entries, 1)
	require.Equal(t, "add login page", byName["Features"].Entries[0].Description)

	require.Contains(t, byName, "Bug Fixes")
	require.Len(t, byName["Bug Fixes"].Entries, 1)
	require.Equal(t, "correct token expiry check", byName["Bug Fixes"].Entries[0].Description)
	require.Equal(t, "auth", byName["Bug Fixes"].Entries[0].Scope)

	require.Contains(t, byName, "Other")
	require.Len(t, byName["Other"].Entries, 1)
	require.Equal(t, "bump deps", byName["Other"].Entries[0].Description)

	// The merge commit isn't a Conventional Commit and must not appear anywhere.
	for _, s := range sections {
		for _, e := range s.Entries {
			require.NotContains(t, e.Description, "Merge pull request")
		}
	}

	require.Contains(t, plan.Rendered, "## Features")
	require.Contains(t, plan.Rendered, "- add login page")
}

func TestCompute_EmptyRepo(t *testing.T) {
	repo := initRepo(t)

	plan, err := engine.Compute(context.Background(), repo, config.Default(), nil)
	require.NoError(t, err)
	require.Empty(t, plan.Packages)
}

func TestCompute_SkipChangelogTrailerExcludesCommit(t *testing.T) {
	repo := initRepo(t,
		"feat: add login page",
		"chore: bump lockfile\n\nSkip-Changelog: true",
	)

	plan, err := engine.Compute(context.Background(), repo, config.Default(), nil)
	require.NoError(t, err)

	sections := soleSections(t, plan)
	require.Len(t, sections, 1)
	require.Equal(t, "Features", sections[0].Name)
}

func TestCompute_SkipChangelogTagExcludesCommit(t *testing.T) {
	repo := initRepo(t,
		"feat: add login page",
		"chore: bump lockfile [skip changelog]",
	)

	plan, err := engine.Compute(context.Background(), repo, config.Default(), nil)
	require.NoError(t, err)

	sections := soleSections(t, plan)
	require.Len(t, sections, 1)
	require.Equal(t, "Features", sections[0].Name)
}

func TestCompute_CustomCategoriesAndTrailerKey(t *testing.T) {
	repo := initRepo(t,
		"feat: add login page",
		"docs: write install guide",
		"chore: bump lockfile\n\nIgnore-Me: true",
	)

	cfg := &config.Config{
		Categories: []config.Category{
			{Type: "docs", Section: "Documentation"},
			{Type: "feat", Section: "New Stuff"},
		},
		SkipChangelogTrailer: "Ignore-Me",
		TagFormat:            config.Default().TagFormat,
		Template:             config.Default().Template,
	}

	plan, err := engine.Compute(context.Background(), repo, cfg, nil)
	require.NoError(t, err)

	sections := soleSections(t, plan)
	require.Len(t, sections, 2)
	require.Equal(t, "Documentation", sections[0].Name, "custom order puts docs first")
	require.Equal(t, "New Stuff", sections[1].Name)
}

func TestCompute_ScopeSpecificCategoryTakesPrecedence(t *testing.T) {
	repo := initRepo(t,
		"fix(security): patch auth bypass",
		"fix: correct pagination off-by-one",
		"chore(deps): bump lockfile",
		"chore: tidy up build script",
	)

	cfg := &config.Config{
		Categories: []config.Category{
			{Type: "fix", Scope: "security", Section: "Security"},
			{Type: "fix", Section: "Bug Fixes"},
			{Type: "chore", Scope: "deps", Section: "Dependency Updates"},
			{Type: "chore", Section: "Maintenance"},
		},
		TagFormat: config.Default().TagFormat,
		Template:  config.Default().Template,
	}

	plan, err := engine.Compute(context.Background(), repo, cfg, nil)
	require.NoError(t, err)

	sections := soleSections(t, plan)
	byName := make(map[string]engine.Section, len(sections))
	for _, s := range sections {
		byName[s.Name] = s
	}

	require.Contains(t, byName, "Security")
	require.Len(t, byName["Security"].Entries, 1)
	require.Equal(t, "patch auth bypass", byName["Security"].Entries[0].Description)

	require.Contains(t, byName, "Bug Fixes")
	require.Len(t, byName["Bug Fixes"].Entries, 1)
	require.Equal(t, "correct pagination off-by-one", byName["Bug Fixes"].Entries[0].Description)

	require.Contains(t, byName, "Dependency Updates")
	require.Len(t, byName["Dependency Updates"].Entries, 1)
	require.Equal(t, "bump lockfile", byName["Dependency Updates"].Entries[0].Description)

	require.Contains(t, byName, "Maintenance")
	require.Len(t, byName["Maintenance"].Entries, 1)
	require.Equal(t, "tidy up build script", byName["Maintenance"].Entries[0].Description)
}

func TestCompute_ScopeSpecificRuleListedAfterGenericNeverMatches(t *testing.T) {
	// If a broader type-only rule for the same Type is listed first, it
	// always wins — a later scope-specific rule for the same Type is
	// unreachable. This documents that ordering, rather than specificity,
	// decides precedence (config.Category's doc comment).
	repo := initRepo(t, "fix(security): patch auth bypass")

	cfg := &config.Config{
		Categories: []config.Category{
			{Type: "fix", Section: "Bug Fixes"},
			{Type: "fix", Scope: "security", Section: "Security"},
		},
		TagFormat: config.Default().TagFormat,
		Template:  config.Default().Template,
	}

	plan, err := engine.Compute(context.Background(), repo, cfg, nil)
	require.NoError(t, err)

	sections := soleSections(t, plan)
	require.Len(t, sections, 1)
	require.Equal(t, "Bug Fixes", sections[0].Name)
}

func monorepoConfig() *config.Config {
	return &config.Config{
		Categories: config.Default().Categories,
		Packages: []config.Package{
			{Path: "packages/api", Name: "api"},
			{Path: "packages/web", Name: "web"},
		},
		SkipChangelogTrailer: config.Default().SkipChangelogTrailer,
		TagFormat:            config.Default().TagFormat,
		Template:             config.Default().Template,
	}
}

func TestCompute_SinglePackagePrefix_UnrelatedCommitUnmapped(t *testing.T) {
	repo := initRepoAt(t,
		commitAt{message: "feat: add endpoint", files: []string{"packages/api/handler.go"}},
		commitAt{message: "chore: root readme", files: []string{"README.md"}},
	)

	plan, err := engine.Compute(context.Background(), repo, monorepoConfig(), nil)
	require.NoError(t, err)

	require.Len(t, plan.Packages, 1, "the root-only commit matches no configured package and is dropped")
	require.Equal(t, "api", plan.Packages[0].Name)
	require.Len(t, plan.Packages[0].Sections, 1)
	require.Equal(t, "add endpoint", plan.Packages[0].Sections[0].Entries[0].Description)
}

func TestCompute_MultiPackage_NonOverlappingCommits(t *testing.T) {
	repo := initRepoAt(t,
		commitAt{message: "feat: add endpoint", files: []string{"packages/api/handler.go"}},
		commitAt{message: "fix: correct layout", files: []string{"packages/web/layout.js"}},
	)

	plan, err := engine.Compute(context.Background(), repo, monorepoConfig(), nil)
	require.NoError(t, err)

	require.Len(t, plan.Packages, 2)
	require.Equal(t, "api", plan.Packages[0].Name, "package order follows config declaration order")
	require.Equal(t, "web", plan.Packages[1].Name)
	require.Equal(t, "add endpoint", plan.Packages[0].Sections[0].Entries[0].Description)
	require.Equal(t, "correct layout", plan.Packages[1].Sections[0].Entries[0].Description)
}

func TestCompute_CrossCuttingCommitDuplicatesIntoEveryPackage(t *testing.T) {
	repo := initRepoAt(t,
		commitAt{
			message: "feat: shared auth flow",
			files:   []string{"packages/api/auth.go", "packages/web/auth.js"},
		},
	)

	plan, err := engine.Compute(context.Background(), repo, monorepoConfig(), nil)
	require.NoError(t, err)

	require.Len(t, plan.Packages, 2)
	for _, pkg := range plan.Packages {
		require.Len(t, pkg.Sections, 1)
		require.Len(t, pkg.Sections[0].Entries, 1)
		require.Equal(t, "shared auth flow", pkg.Sections[0].Entries[0].Description,
			"the same Entry content appears in both packages, duplicated")
	}
}

func TestCompute_NoPriorTag_FullHistoryAndBootstrapVersion(t *testing.T) {
	dir := newRepo(t)
	commitMessage(t, dir, "feat: add login page", 0)
	commitMessage(t, dir, "fix: correct typo", 1)

	plan, err := engine.Compute(context.Background(), dir, config.Default(), nil)
	require.NoError(t, err)

	require.Empty(t, plan.PreviousVersion)
	require.Equal(t, "0.1.0", plan.SuggestedVersion, "first release bootstraps regardless of bump severity")

	sections := soleSections(t, plan)
	require.Len(t, sections, 2, "no prior tag means the full history is in range")
}

func TestCompute_PriorMatchingTag_BoundsRangeAndIncrementsVersion(t *testing.T) {
	dir := newRepo(t)
	commitMessage(t, dir, "feat: add login page", 0)
	tagRepo(t, dir, "v1.0.0")
	commitMessage(t, dir, "fix: correct typo", 1)

	plan, err := engine.Compute(context.Background(), dir, config.Default(), nil)
	require.NoError(t, err)

	require.Equal(t, "1.0.0", plan.PreviousVersion)
	require.Equal(t, "1.0.1", plan.SuggestedVersion, "only the fix after v1.0.0 is in range")

	sections := soleSections(t, plan)
	require.Len(t, sections, 1)
	require.Equal(t, "Bug Fixes", sections[0].Name)
	require.Equal(t, "correct typo", sections[0].Entries[0].Description)
}

func TestCompute_NonMatchingTagIsIgnored(t *testing.T) {
	dir := newRepo(t)
	commitMessage(t, dir, "feat: add login page", 0)
	tagRepo(t, dir, "checkpoint-1") // doesn't match the default "v{{version}}" format
	commitMessage(t, dir, "fix: correct typo", 1)

	plan, err := engine.Compute(context.Background(), dir, config.Default(), nil)
	require.NoError(t, err)

	require.Empty(t, plan.PreviousVersion, "the non-matching tag isn't treated as a previous release")
	require.Equal(t, "0.1.0", plan.SuggestedVersion)

	sections := soleSections(t, plan)
	require.Len(t, sections, 2, "history isn't bounded by a tag that doesn't match tag-format")
}

func TestCompute_HighestSeverityWins(t *testing.T) {
	dir := newRepo(t)
	commitMessage(t, dir, "fix: correct typo", 0)
	commitMessage(t, dir, "feat!: remove deprecated endpoint", 1)
	tagRepo(t, dir, "v1.0.0")

	plan, err := engine.Compute(context.Background(), dir, config.Default(), nil)
	require.NoError(t, err)
	require.Empty(t, plan.SuggestedVersion, "no commits after the tag, so no release is suggested")

	commitMessage(t, dir, "fix: another patch", 2)
	commitMessage(t, dir, "feat: minor addition", 3)
	commitMessage(t, dir, "feat!: breaking change", 4)

	plan, err = engine.Compute(context.Background(), dir, config.Default(), nil)
	require.NoError(t, err)
	require.Equal(t, "2.0.0", plan.SuggestedVersion, "breaking beats feat and fix")
}

func TestCompute_NoBumpWorthyEntries_NoSuggestion(t *testing.T) {
	dir := newRepo(t)
	commitMessage(t, dir, "feat: add login page", 0)
	tagRepo(t, dir, "v1.0.0")
	commitMessage(t, dir, "docs: update readme", 1)

	plan, err := engine.Compute(context.Background(), dir, config.Default(), nil)
	require.NoError(t, err)

	require.Equal(t, "1.0.0", plan.PreviousVersion)
	require.Empty(t, plan.SuggestedVersion, "docs alone doesn't warrant a release")
}

func TestCompute_Chore_BumpsPatch(t *testing.T) {
	dir := newRepo(t)
	commitMessage(t, dir, "feat: add login page", 0)
	tagRepo(t, dir, "v1.0.0")
	commitMessage(t, dir, "chore: bump deps", 1)

	plan, err := engine.Compute(context.Background(), dir, config.Default(), nil)
	require.NoError(t, err)

	require.Equal(t, "1.0.0", plan.PreviousVersion)
	require.Equal(t, "1.0.1", plan.SuggestedVersion, "chore warrants a patch release")
}

func TestCompute_AttachesPRReferenceFromCommitText(t *testing.T) {
	dir := newRepo(t)
	commitMessage(t, dir, "feat: add login page (#123)", 0)
	commitMessage(t, dir,
		"fix(auth): rotate keys\n\nReviewed-on: https://gitea.example.com/brpaz/draftsman/pulls/9", 1)
	commitMessage(t, dir, "chore: bump deps", 2)

	plan, err := engine.Compute(context.Background(), dir, config.Default(), nil)
	require.NoError(t, err)

	sections := soleSections(t, plan)
	byName := map[string]engine.Section{}
	for _, s := range sections {
		byName[s.Name] = s
	}

	require.NotNil(t, byName["Features"].Entries[0].PR)
	require.Equal(t, 123, byName["Features"].Entries[0].PR.Number)
	require.Empty(t, byName["Features"].Entries[0].PR.Link, "github (#N) form carries no URL in text")
	require.Equal(t, "add login page", byName["Features"].Entries[0].Description,
		"the (#123) suffix is stripped from Description so it isn't duplicated by template rendering")

	require.NotNil(t, byName["Bug Fixes"].Entries[0].PR)
	require.Equal(t, 9, byName["Bug Fixes"].Entries[0].PR.Number)
	require.Equal(t, "https://gitea.example.com/brpaz/draftsman/pulls/9", byName["Bug Fixes"].Entries[0].PR.Link)

	require.Nil(t, byName["Other"].Entries[0].PR, "chore commit has no PR reference in text")

	require.Contains(t, plan.Rendered, "- add login page (#123)")
	require.Contains(t, plan.Rendered,
		"- rotate keys ([#9](https://gitea.example.com/brpaz/draftsman/pulls/9))")
}

func multiModeConfig() *config.Config {
	return &config.Config{
		Mode:       config.ModeMulti,
		Categories: config.Default().Categories,
		Packages: []config.Package{
			{Path: "packages/api", Name: "api"},
			{Path: "packages/web", Name: "web"},
		},
		SkipChangelogTrailer: config.Default().SkipChangelogTrailer,
		TagFormat:            "{{package}}-v{{version}}",
		Template:             config.Default().Template,
	}
}

func TestCompute_MultiMode_IndependentVersionsAndRanges(t *testing.T) {
	dir := newRepo(t)

	commitFiles(t, dir, "feat: add endpoint", 0, "packages/api/handler.go")
	tagRepo(t, dir, "api-v1.0.0")

	commitFiles(t, dir, "fix: correct layout", 1, "packages/web/layout.js")
	tagRepo(t, dir, "web-v1.0.0")

	// After api's tag: a fix touching only api.
	commitFiles(t, dir, "fix: handle nil", 2, "packages/api/handler.go")
	// After web's tag: a feat touching only web.
	commitFiles(t, dir, "feat: dark mode", 3, "packages/web/layout.js")

	plan, err := engine.Compute(context.Background(), dir, multiModeConfig(), nil)
	require.NoError(t, err)

	require.Len(t, plan.Packages, 2)
	byName := map[string]engine.PackagePlan{}
	for _, p := range plan.Packages {
		byName[p.Name] = p
	}

	api := byName["api"]
	require.Equal(t, "1.0.0", api.PreviousVersion)
	require.Equal(t, "1.0.1", api.SuggestedVersion, "only the fix after api-v1.0.0 is in api's range")
	require.Len(t, api.Sections, 1)
	require.Equal(t, "handle nil", api.Sections[0].Entries[0].Description)

	web := byName["web"]
	require.Equal(t, "1.0.0", web.PreviousVersion)
	require.Equal(t, "1.1.0", web.SuggestedVersion, "only the feat after web-v1.0.0 is in web's range")
	require.Len(t, web.Sections, 1)
	require.Equal(t, "dark mode", web.Sections[0].Entries[0].Description)

	require.Contains(t, plan.Rendered, "# api (1.0.1)")
	require.Contains(t, plan.Rendered, "# web (1.1.0)")
}

func TestCompute_MultiMode_PackageWithNoChangesSinceItsTagIsOmitted(t *testing.T) {
	dir := newRepo(t)

	commitFiles(t, dir, "feat: add endpoint", 0, "packages/api/handler.go")
	tagRepo(t, dir, "api-v1.0.0")
	commitFiles(t, dir, "feat: add page", 1, "packages/web/index.js")
	tagRepo(t, dir, "web-v1.0.0")

	// Nothing changes in api after its tag.
	commitFiles(t, dir, "fix: layout tweak", 2, "packages/web/index.js")

	plan, err := engine.Compute(context.Background(), dir, multiModeConfig(), nil)
	require.NoError(t, err)

	require.Len(t, plan.Packages, 1, "api has no Entries since its own last tag and is omitted")
	require.Equal(t, "web", plan.Packages[0].Name)
}

func TestCompute_PRFallback_TextExtractionFirstThenBackend(t *testing.T) {
	dir := newRepo(t)
	commitMessage(t, dir, "feat: add login page (#123)", 0) // text-extractable
	commitMessage(t, dir, "fix: correct typo", 1)           // not — fallback should fire

	var resolvedSHAs []string
	fake := &fakeBackend{
		resolvePR: func(_ context.Context, sha string) (commit.PRReference, bool, error) {
			resolvedSHAs = append(resolvedSHAs, sha)
			return commit.PRReference{Number: 99, Link: "https://example.com/pull/99"}, true, nil
		},
	}

	plan, err := engine.Compute(context.Background(), dir, config.Default(), fake)
	require.NoError(t, err)

	sections := soleSections(t, plan)
	byName := map[string]engine.Section{}
	for _, s := range sections {
		byName[s.Name] = s
	}

	require.Equal(t, 123, byName["Features"].Entries[0].PR.Number,
		"text extraction wins even when a fallback is available")
	require.Equal(t, 99, byName["Bug Fixes"].Entries[0].PR.Number,
		"fallback fills in when text extraction found nothing")
	require.Len(t, resolvedSHAs, 1, "fallback is only called for the commit with no text-extractable reference")
}

func TestCompute_NoBackend_NoFallbackAttempted(t *testing.T) {
	dir := newRepo(t)
	commitMessage(t, dir, "fix: correct typo", 0)

	plan, err := engine.Compute(context.Background(), dir, config.Default(), nil)
	require.NoError(t, err)

	sections := soleSections(t, plan)
	require.Nil(t, sections[0].Entries[0].PR, "no backend means no fallback, same as before this ticket")
}
