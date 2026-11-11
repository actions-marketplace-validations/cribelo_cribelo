package render

import (
	"strings"
	"testing"

	engine "github.com/ast-metrics/ast-metrics/cribelo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// report wraps a plan the way the command line does, without links.
func report(p *engine.Plan) Report { return Report{Plan: p} }

func samplePlan() Report {
	return report(&engine.Plan{
		Methodology: engine.MethodologyVersion, Base: "main", BaseSha: "abcdef0123456", HeadSha: "fedcba",
		Changed: 3, ReadCount: 1, ReadMinutes: 2, ChangeMinutes: 3,
		Gate: "failed", FailOn: []string{engine.FactCycle}, FailOverMinutes: 2, GateReasons: []string{"reading time 3 min, over the 2 min allowed", "a.go: introduces a cycle: a.go → b.go → a.go"},
		Files: []engine.FileReport{
			{Path: "a.go", Status: "M", Added: 10, Deleted: 2, Score: 0.81, Level: engine.LevelHigh, Minutes: 2, Read: true, Reasons: []string{"imported by 4 files", "no test touched"},
				Values:        map[engine.Feature]float64{engine.FeatureMaxComplexity: 5, engine.FeatureUntested: 1, engine.FeatureAfferent: 4, engine.FeatureChurn90d: 7, engine.FeatureFixes90d: 2},
				Contributions: []engine.Contribution{{Feature: engine.FeatureAfferent, Value: 4, Logit: 1.2}, {Feature: engine.FeatureClone, Value: 0.9, Logit: 1.35}},
				Functions:     []engine.TouchedFunction{{Name: "F", Line: 3, Before: 1, After: 5}, {Name: "G", Line: 9, New: true, After: 2}},
				Clone:         &engine.CloneMatch{Path: "b.go", Function: "F ~ H", Line: 7, Similarity: 0.9}},
			{Path: "c.go", Score: 0.1, Level: engine.LevelLow, Minutes: 1, Reasons: []string{"+2 lines"}},
		},
		Skipped: []engine.Skipped{{Path: "c.go", Kind: "code", Reason: "no measurable change", Score: 0.1}, {Path: "README.md", Kind: "not analyzed", Reason: "not analyzed (docs, config, other languages)"}},
	})
}

func TestPlanText(t *testing.T) {
	SetColor(false)
	text := samplePlan().Text()
	assert.Contains(t, text, "3 files changed vs main (merge-base abcdef0)")
	assert.Contains(t, text, "Read 1 of 3 changed files, about 2 min.")
	assert.Contains(t, text, "lines to read closely  lines to glance at\n  1  a.go\n")
	assert.Contains(t, text, "Why these files\n           complexity  used by  commits 90d       clone  test changed\n")
	assert.Contains(t, text, "  1  a.go  ▮▯▯▯▯ 5     ▮▮▯▯▯ 4  ▮▮▮▯▯ 7, 2 fixes  90%    ✗\n")
	assert.Contains(t, text, "Totals  12 lines to read closely, 0 to glance at (20 and 60 lines a minute)\n")
	assert.Contains(t, text, "Skip (2)")
	assert.Contains(t, text, "Gate: failed\n  reading time 3 min, over the 2 min allowed\n  a.go: introduces a cycle")
	assert.NotContains(t, text, "high, 0 medium")
	assert.Contains(t, text, "generic, not yet calibrated")
}

func TestPlanTextEmptyAndNoCode(t *testing.T) {
	SetColor(false)
	assert.Contains(t, report(&engine.Plan{BaseSha: "abc"}).Text(), "Nothing changed.")
	assert.Contains(t, report(&engine.Plan{Changed: 2, BaseSha: "abc"}).Text(), "Nothing to read")
	assert.NotContains(t, report(&engine.Plan{Changed: 2, BaseSha: "abc"}).Text(), "Gate")
	passed := report(&engine.Plan{Changed: 2, BaseSha: "abc", Gate: "passed", FailOn: []string{engine.FactCycle}, FailOverMinutes: 60, ChangeMinutes: 48})
	assert.Contains(t, passed.Text(), "Gate: passed (reading time 48 min of 1 h allowed, no cycle)")
	assert.Contains(t, passed.Markdown(), "**Gate:** passed · reading time 48 min of 1 h allowed, no cycle")
}

func TestPlanMarkdown(t *testing.T) {
	md := samplePlan().Markdown()
	assert.Contains(t, md, "## Cribelo: read 1 of 3 changed files, about 2 min\n")
	assert.Contains(t, md, "| | File | Lines to read&nbsp;closely | Lines to glance&nbsp;at |\n|--:|:--|:--|:--|\n| 1 | `a.go` |  |  |\n")
	assert.Contains(t, md, "<details open><summary><b>Why these files</b></summary>\n\n| | File | Complexity | Used&nbsp;by | Commits&nbsp;90d | Test changed |\n|--:|:--|:--|:--|:--|:-:|\n")
	assert.Contains(t, md, "| 1 | `a.go`<br><sub>90% similar to `b.go:H()`</sub> | 🟩&#8288;⬜&#8288;⬜&#8288;⬜&#8288;⬜&nbsp;5 | 🟨&#8288;🟨&#8288;⬜&#8288;⬜&#8288;⬜&nbsp;4 | 🟧&#8288;🟧&#8288;🟧&#8288;⬜&#8288;⬜&nbsp;7, 2&nbsp;fixes | ❌ |\n")
	assert.Contains(t, md, "<sub><b>Complexity</b>: highest cyclomatic complexity among the functions touched.")
	assert.Contains(t, md, "<details><summary><b>Totals</b>: 12 lines to read closely, 0 to glance at</summary>\n\n| | File | Lines to read closely | Lines to glance at |\n|--:|:--|--:|--:|\n| 1 | `a.go` | 12 | 0 |\n\n<sub><b>Read closely</b>: code lines in functions that gained a branch, or whose complexity is above 3.")
	assert.Contains(t, md, "20 lines a minute read closely, 60 glanced at")
	assert.NotContains(t, md, "**Total**", "one file needs no total")
	assert.Contains(t, md, "<details><summary><b>Skip</b> (2 files)")
	assert.Contains(t, md, "- `README.md`")
	assert.Contains(t, md, "**Gate:** failed · reading time 3 min, over the 2 min allowed · a.go: introduces a cycle")
	assert.Contains(t, md, "<sub>cribelo dev · methodology "+engine.MethodologyVersion)
	assert.Contains(t, Report{Plan: samplePlan().Plan, Version: "1.2.3"}.Markdown(), "<sub>cribelo 1.2.3 · methodology")

	assert.NotContains(t, md, "—")
	assert.Contains(t, report(&engine.Plan{}).Markdown(), "nothing changed")
	assert.Contains(t, report(&engine.Plan{Changed: 2}).Markdown(), "nothing to read (2 files changed, no code)")

	p := samplePlan()
	p.Calibrated, p.TrainedOn, p.TrainedAt = true, 120, "2026-01-01"
	assert.Contains(t, p.Markdown(), "calibrated on this repository's last 120 changes (2026-01-01)")
}

func TestSkipAndSummary(t *testing.T) {
	SetColor(false)
	p := samplePlan()
	text := p.Skip()
	assert.Contains(t, text, "no measurable change (score 0.10)")
	assert.Contains(t, text, "README.md")
	assert.Equal(t, "Nothing skipped.\n", report(&engine.Plan{}).Skip())

	skipped := []engine.Skipped{{Reason: "b"}, {Reason: "a"}, {Reason: "b"}, {Reason: "c"}}
	assert.Equal(t, "2 b | 1 a | 1 c", skipSummary(skipped, " | "), "count desc, then name")
}

func TestPlanExplainUnit(t *testing.T) {
	SetColor(false)
	p := samplePlan()
	out, err := p.Explain("a.go")
	require.NoError(t, err)
	assert.Contains(t, out, "score 0.81")
	assert.Contains(t, out, "in the reading list")
	assert.Contains(t, out, "complexity 1 → 5")
	assert.Contains(t, out, "new, complexity 2")
	assert.Contains(t, out, "Possible duplicate")
	assert.Contains(t, out, "90% similar to b.go:7")

	out, err = p.Explain("README.md")
	require.NoError(t, err)
	assert.Contains(t, out, "README.md is skipped")

	_, err = p.Explain("nope.go")
	assert.Error(t, err)
}

func TestBars(t *testing.T) {
	assert.Equal(t, 0, cells(0, complexitySteps))
	assert.Equal(t, 1, cells(5, complexitySteps))
	assert.Equal(t, 3, cells(16, complexitySteps))
	assert.Equal(t, 4, cells(25, complexitySteps))
	assert.Equal(t, 5, cells(900, commitSteps))
	assert.Equal(t, "🟧&#8288;🟧&#8288;🟧&#8288;⬜&#8288;⬜&nbsp;16", barCell(16, complexitySteps).markdown, "a bar and its value never break")
	assert.Equal(t, "⬜&#8288;⬜&#8288;⬜&#8288;⬜&#8288;⬜", barCell(0, complexitySteps).markdown)
	assert.Equal(t, "▮▮▮▮▮ 16", barCell(16, complexitySteps).plain)
}

// readingPlan is a file, its test, a file that is only glanced at, and a
// file of the same name as the first.
func readingPlan() Report {
	return report(&engine.Plan{
		Changed: 5, ReadCount: 4, ReadMinutes: 9,
		Files: []engine.FileReport{
			{Path: "core/store.go", Status: "A", Read: true, ToRead: 40, ToGlance: 12, Hunks: []engine.HunkReport{{}},
				Ranges:       []engine.LineRange{{Start: 10, End: 14}, {Start: 30, End: 30}, {Start: 40, End: 62}, {Start: 70, End: 71}, {Start: 80, End: 90}, {Start: 95, End: 99}},
				GlanceRanges: []engine.LineRange{{Start: 3, End: 8}},
				Values:       map[engine.Feature]float64{engine.FeatureMaxComplexity: 7, engine.FeatureAfferent: 2}},
			{Path: "core/store_test.go", Read: true, Test: true, ToRead: 10, ToGlance: 30, Hunks: []engine.HunkReport{{}},
				Ranges: []engine.LineRange{{Start: 8, End: 8}}, GlanceRanges: []engine.LineRange{{Start: 20, End: 49}}, Values: map[engine.Feature]float64{}},
			{Path: "api/handler.go", Status: "M", Read: true, ToRead: 0, ToGlance: 6, Hunks: []engine.HunkReport{{}},
				GlanceRanges: []engine.LineRange{{Start: 12, End: 12}},
				Values:       map[engine.Feature]float64{engine.FeatureMaxComplexity: 12, engine.FeatureUntested: 1}},
			{Path: "cmd/store.go", Status: "M", Read: true, ToRead: 3, ToGlance: 0, Hunks: []engine.HunkReport{{}},
				Ranges: []engine.LineRange{{Start: 5, End: 7}}, Values: map[engine.Feature]float64{},
				Clone: &engine.CloneMatch{Path: "pkg/core/store.go", Function: "run ~ Store::Put", Similarity: 0.84}},
			{Path: "left.go"},
		},
		Groups: []engine.ReadingGroup{
			{Files: []string{"core/store.go", "core/store_test.go", "api/handler.go"}, Linked: true},
			{Files: []string{"cmd/store.go"}},
		},
	})
}

func TestReadingText(t *testing.T) {
	SetColor(false)
	text := readingPlan().Text()
	assert.Contains(t, text, ""+
		"                               lines to read closely             lines to glance at\n"+
		"  1  core/store.go       new   10-14, 30, 40-62, 70-71, +2 more  3-8\n"+
		"  2  core/store_test.go  test  line 8                            20-49\n"+
		"  3  api/handler.go                                              line 12\n"+
		"  4  cmd/store.go              5-7\n", "a lone line is called one, so that it is not taken for a count")
	assert.Contains(t, text, "Why these files\n"+
		"                    complexity  used by  clone  test changed\n"+
		"  1  core/store.go  ▮▮▯▯▯ 7     ▮▯▯▯▯ 2         ✓\n"+
		"  3  handler.go     ▮▮▮▯▯ 12    ▯▯▯▯▯           ✗\n"+
		"  4  cmd/store.go   ▯▯▯▯▯       ▯▯▯▯▯    84%    ✓\n", "tests carry no signal, two files of the same name keep a directory")
	assert.Contains(t, text, "Totals  53 lines to read closely, 48 to glance at (20 and 60 lines a minute)\n")
	assert.NotContains(t, text, "↳")
}

func TestReadingMarkdown(t *testing.T) {
	p := readingPlan()
	md := p.Markdown()
	assert.Contains(t, md, "## Cribelo: read 4 of 5 changed files, about 9 min\n\n"+
		"| | File | Lines to read&nbsp;closely | Lines to glance&nbsp;at |\n|--:|:--|:--|:--|\n"+
		"| 1 | `core/store.go` *new* | 10-&#8288;14, 30, 40-&#8288;62, 70-&#8288;71, +2&nbsp;more | 3-&#8288;8 |\n"+
		"| 2 | `core/store_test.go` *test* | line&nbsp;8 | 20-&#8288;49 |\n"+
		"| 3 | `api/handler.go` |  | line&nbsp;12 |\n"+
		"| 4 | `cmd/store.go` | 5-&#8288;7 |  |\n\n"+
		"<details open><summary><b>Why these files</b></summary>\n\n"+
		"| | File | Complexity | Used&nbsp;by | Test changed |\n")
	assert.Contains(t, md, "| 4 | `cmd/store.go`<br><sub>84% similar to `pkg/core/store.go:Put()`</sub> | ⬜&#8288;⬜&#8288;⬜&#8288;⬜&#8288;⬜ | ⬜&#8288;⬜&#8288;⬜&#8288;⬜&#8288;⬜ | ✅ |\n")
	assert.NotContains(t, md, "| 2 | `store_test.go` |", "tests carry no signal")
	assert.Contains(t, md, "</details>\n\n<details><summary><b>Totals</b>: 53 lines to read closely, 48 to glance at</summary>")
	assert.Contains(t, md, "| 2 | `core/store_test.go` | 10 | 30 |\n")
	assert.Contains(t, md, "| | **Total** | **53** | **48** |\n")

	links, err := PullRequestLinks("https://github.com/o/r/pull/12/files", "abc")
	require.NoError(t, err)
	p.Links = links
	md = p.Markdown()
	assert.Regexp(t, "\\| 2 \\| `core/store_test.go` \\*test\\* \\| line&nbsp;\\[8\\]\\(https://github.com/o/r/pull/12/files#diff-[0-9a-f]{64}R8\\) \\| \\[20-&#8288;49\\]\\(https://github.com/o/r/pull/12/files#diff-[0-9a-f]{64}R20-R49\\) \\|\n", md)
	SetColor(false)
	assert.NotContains(t, p.Text(), "https://", "a terminal gets no link")
}

func TestPullRequestLinks(t *testing.T) {
	github, err := PullRequestLinks("https://github.com/ast-metrics/ast-metrics/pull/209", "abc")
	require.NoError(t, err)
	// The anchor of a file in a GitHub diff is the SHA-256 of its path, as read on pull request 209.
	const file = "https://github.com/ast-metrics/ast-metrics/pull/209/files#diff-29027a7f75bca43b050fc18363db0152271a66e310ca57558ce6da67a71954e4"
	assert.Equal(t, file, github("internal/cribelo/table.go", 0, 0))
	assert.Equal(t, file+"R12", github("internal/cribelo/table.go", 12, 12))
	assert.Equal(t, file+"R12-R40", github("internal/cribelo/table.go", 12, 40))

	gitlab, err := PullRequestLinks("https://gitlab.example.com/group/sub/project/-/merge_requests/7", "abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://gitlab.example.com/group/sub/project/-/blob/abc123/src/a%20b.go", gitlab("src/a b.go", 0, 0))
	assert.Equal(t, "https://gitlab.example.com/group/sub/project/-/blob/abc123/src/a.go#L12-40", gitlab("src/a.go", 12, 40))
	assert.Equal(t, "https://gitlab.example.com/group/sub/project/-/blob/abc123/src/a.go#L12", gitlab("src/a.go", 12, 12))

	_, err = PullRequestLinks("https://example.com/somewhere", "abc")
	assert.Error(t, err)
}

func TestShortNames(t *testing.T) {
	names := shortNames([]string{"a/deps/resolver.go", "b/deps/resolver.go", "a/util.go", "resolver.go"})
	assert.Equal(t, map[string]string{"a/deps/resolver.go": "a/deps/resolver.go", "b/deps/resolver.go": "b/deps/resolver.go", "a/util.go": "util.go", "resolver.go": "resolver.go"}, names)
	assert.Equal(t, "c/d.go", tail("a/b/c/d.go", 2))
	assert.Equal(t, "d.go", tail("d.go", 3))
}

func TestFacts(t *testing.T) {
	p := samplePlan()
	p.Facts = []engine.Fact{{Kind: engine.FactDependency, Path: "a.go", Text: "core depends on api for the first time"}}
	assert.Contains(t, p.Markdown(), "- `a.go`: core depends on api for the first time\n")
	SetColor(false)
	assert.Contains(t, p.Text(), "Worth knowing  true by construction, not predicted\n  a.go  core depends on api for the first time\n")
}

func TestNewFileIsTagged(t *testing.T) {
	SetColor(false)
	p := report(&engine.Plan{Changed: 1, ReadCount: 1, Files: []engine.FileReport{{Path: "n.go", Status: "A", Added: 3, Read: true, Reasons: []string{"new file"}}}})
	assert.Contains(t, p.Text(), "n.go  new")
	assert.Contains(t, p.Markdown(), "| 1 | `n.go` *new* |  |  |\n")
}

func readReport(path string, score float64, toRead int) engine.FileReport {
	return engine.FileReport{Path: path, Score: score, ToRead: toRead, Added: toRead, Minutes: 1, Read: true, Hunks: []engine.HunkReport{{Nature: engine.NatureBehavior}}}
}

func TestTextShowsGroupsAndTests(t *testing.T) {
	SetColor(false)
	test := engine.FileReport{Path: "app/store_test.go", Minutes: 1, ToRead: 5, Added: 5, Read: true, Test: true}
	p := report(&engine.Plan{Changed: 4, ReadCount: 4, ReadMinutes: 4, LinesChanged: 125, LinesToRead: 125,
		Files:  []engine.FileReport{readReport("app/store.go", 0.5, 30), test, readReport("app/handler.go", 0.9, 80), readReport("other/x.go", 0.7, 10)},
		Groups: []engine.ReadingGroup{{Files: []string{"app/store.go", "app/store_test.go", "app/handler.go"}, Linked: true}, {Files: []string{"other/x.go"}}}})
	text := p.Text()
	assert.Regexp(t, `store_test\.go\s+test`, text)
	assert.Less(t, strings.Index(text, "app/store.go"), strings.Index(text, "app/handler.go"))
	assert.Contains(t, p.Markdown(), "| 1 | `app/store.go` |  |  |\n| 2 | `app/store_test.go` *test* |  |  |\n| 3 | `app/handler.go` |  |  |\n| 4 | `other/x.go` |  |  |\n")
}

func TestTextAllMechanical(t *testing.T) {
	SetColor(false)
	p := report(&engine.Plan{Changed: 1, Files: []engine.FileReport{{Path: "moved.go", Added: 40, Hunks: []engine.HunkReport{{Nature: engine.NatureMoved}}, Natures: map[engine.Nature]engine.NatureCount{engine.NatureMoved: {Hunks: 1, Lines: 40}}}}, LinesChanged: 40,
		Natures: map[engine.Nature]engine.NatureCount{engine.NatureMoved: {Hunks: 1, Lines: 40}}})
	assert.Contains(t, p.Text(), "every change is mechanical")
	assert.Contains(t, p.Text(), "0 of 40 changed lines to read: 1 move.")
	assert.Contains(t, p.Markdown(), "nothing to read (1 file changed, every change is mechanical)")
}

func TestDescribe(t *testing.T) {
	p := report(&engine.Plan{Changed: 5, LinesChanged: 200, LinesToRead: 150, Natures: map[engine.Nature]engine.NatureCount{engine.NatureRename: {Hunks: 4, Lines: 50}},
		Files:     []engine.FileReport{readReport("app/store.go", 0.5, 30), readReport("app/handler.go", 0.9, 80)},
		Groups:    []engine.ReadingGroup{{Files: []string{"app/store.go", "app/handler.go"}, Linked: true}},
		ReadCount: 2, ReadMinutes: 8,
		Facts:   []engine.Fact{{Kind: engine.FactSignature, Path: "app/store.go", Text: "Load() changed its parameters (id → id, ctx), called by name from 3 files"}, {Kind: engine.FactCycle, Path: "app/handler.go", Text: "introduces a cycle: handler.go → store.go → handler.go"}},
		Renames: []engine.Rename{{Pair: "old → new", Hunks: 4}},
		Skipped: []engine.Skipped{{Path: "README.md", Kind: "not analyzed", Reason: "not analyzed (docs, config, other languages)"}}})
	out := p.Describe()
	assert.Contains(t, out, "5 files changed, 200 lines, 150 to read closely.\nSet aside: 4 renames")
	assert.Contains(t, out, "**Signatures and API**")
	assert.Contains(t, out, "- `app/store.go`: Load() changed its parameters")
	assert.Contains(t, out, "**Structure**")
	assert.Contains(t, out, "- `old → new` in 4 hunks")
	assert.Contains(t, out, "1. `app/store.go`: the others use it\n2. `app/handler.go`\n")
	assert.Contains(t, out, "Left out: 1 not analyzed")
	assert.Equal(t, "Nothing changed.\n", report(&engine.Plan{}).Describe())
}

func TestWhyTheseFilesIsFoldedOnlyOnALongList(t *testing.T) {
	p := readingPlan()
	assert.Contains(t, p.Markdown(), "<details open><summary><b>Why these files</b></summary>", "three files with signals: shown")

	p.Files = append(p.Files, engine.FileReport{Path: "cmd/list.go", Status: "M", Read: true, ToRead: 4, Hunks: []engine.HunkReport{{}},
		Ranges: []engine.LineRange{{Start: 2, End: 5}}, Values: map[engine.Feature]float64{engine.FeatureMaxComplexity: 5}})
	md := p.Markdown()
	assert.Contains(t, md, "<details><summary><b>Why these files</b></summary>", "four files with signals: folded")
	assert.NotContains(t, md, "<details open>")
}
