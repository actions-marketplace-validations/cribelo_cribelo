package render

import (
	"encoding/json"
	"strings"
	"testing"

	engine "github.com/ast-metrics/ast-metrics/cribelo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReview(t *testing.T) {
	p := readingPlan()
	p.HeadSha = "fedcba9876"
	p.Facts = []engine.Fact{{Kind: engine.FactCycle, Path: "core/store.go", Text: "introduces a cycle: store.go → api.go → store.go"}}
	store := p.File("core/store.go")
	store.Functions = []engine.TouchedFunction{
		{Name: "Store::Put", Line: 9, End: 20, Before: 2, After: 6},
		{Name: "Get", Line: 28, End: 35, New: true, After: 3},
		{Name: "List", Line: 38, Before: 4, After: 4},
	}
	p.File("core/store.go").Values[engine.FeatureFixes90d] = 1
	review := p.Review(0)

	assert.Equal(t, "fedcba9876", review.CommitID)
	assert.Equal(t, "COMMENT", review.Event)
	assert.Empty(t, review.Body, "the comments speak, a text would stay in the conversation at each push")
	require.Len(t, review.Comments, 5, "three functions of store.go, then its test, then cmd/store.go; handler.go is only glanced at")

	assert.Equal(t, ReviewComment{Path: "core/store.go", Line: 14, StartLine: 10, Side: "RIGHT", StartSide: "RIGHT",
		Body: "**Read closely.** This change adds 4 branches to `Put()` (complexity 2 → 6). For each new condition, check when it is taken and whether a test takes it.\n\n" +
			"2 files depend on this file and it needed 1 fix in the last 90 days.\n\n" +
			"- introduces a cycle: store.go → api.go → store.go\n\n" + Marker}, review.Comments[0])
	assert.Equal(t, ReviewComment{Path: "core/store.go", Line: 30, Side: "RIGHT",
		Body: "**Read closely.** `Get()` is new (complexity 3), nothing to compare it with. Check the cases it handles, and the ones it leaves out.\n\n" + Marker},
		review.Comments[1], "a lone line has no start, what is said of the file is said once")
	assert.Equal(t, ReviewComment{Path: "core/store.go", Line: 62, StartLine: 40, Side: "RIGHT", StartSide: "RIGHT",
		Body: "**Read closely.** No new branch in `List()` (complexity 4), but the changed lines sit inside existing ones. Check that they hold in every case, not only the one the change was written for. " +
			"The change touches it again on lines 70-71, 80-90 and 95-99.\n\n" + Marker},
		review.Comments[2], "a function without a known end holds the spans after its start, and is told once")
	assert.Equal(t, ReviewComment{Path: "core/store_test.go", Line: 8, Side: "RIGHT", Body: "**Read closely.** Check what this test asserts, not only that it passes.\n\n" + Marker}, review.Comments[3])
	assert.Equal(t, ReviewComment{Path: "cmd/store.go", Line: 7, StartLine: 5, Side: "RIGHT", StartSide: "RIGHT", Body: "**Read closely.**\n\n" + Marker},
		review.Comments[4], "no function known, no words about it")

	out, err := review.JSON()
	require.NoError(t, err)
	var back map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &back))
	assert.Equal(t, "RIGHT", back["comments"].([]any)[0].(map[string]any)["side"])
	assert.NotContains(t, out, `"start_line": 0`, "a single line carries no start")
	assert.NotContains(t, out, `"body": ""`, "no text is sent rather than an empty one")
	assert.Contains(t, out, Marker, "the marker is not escaped")
}

func TestReviewNothingToRead(t *testing.T) {
	review := report(&engine.Plan{Changed: 2}).Review(0)
	assert.Empty(t, review.Comments)
	assert.Contains(t, review.Body, "no lines to read closely")
	out, err := review.JSON()
	require.NoError(t, err)
	assert.Contains(t, out, `"comments": []`, "an empty list, so that a script can count it")
}

func TestReviewWordsWhatToCheck(t *testing.T) {
	fn := func(name string, before, after int, isNew bool) *engine.TouchedFunction {
		return &engine.TouchedFunction{Name: name, Before: before, After: after, New: isNew}
	}
	code, test := &engine.FileReport{}, &engine.FileReport{Test: true}

	assert.Equal(t, "This change adds 1 branch to `Parse()`, which already had many (complexity 12 → 13). Check when the new condition is taken and whether a test takes it.",
		whatToCheck(code, []*engine.TouchedFunction{fn("Parser::Parse", 12, 13, false)}))
	assert.Equal(t, "This change adds branches to `B()` (complexity 3 → 7) and `A()` (complexity 2 → 4). For each new condition, check when it is taken and whether a test takes it.",
		whatToCheck(code, []*engine.TouchedFunction{fn("A", 2, 4, false), fn("B", 3, 7, false)}), "the one with the most branches first")
	assert.Equal(t, "`B()` and `A()` are new (complexity 6 and 2), nothing to compare them with. Check the cases they handle, and the ones they leave out.",
		whatToCheck(code, []*engine.TouchedFunction{fn("A", 0, 2, true), fn("B", 0, 6, true)}))
	assert.Equal(t, "New code, nothing to compare it with. `C()` and `B()` hold most of the branching (complexity 9 and 6): start there.",
		whatToCheck(code, []*engine.TouchedFunction{fn("A", 0, 2, true), fn("B", 0, 6, true), fn("C", 0, 9, true)}))
	assert.Equal(t, "This change removes branches from `Old()` (complexity 8 → 5). Check that the cases they handled cannot happen any more.",
		whatToCheck(code, []*engine.TouchedFunction{fn("Old", 8, 5, false)}))
	assert.Equal(t, "This test has logic of its own: `TestPut()` has complexity 5. Check what it asserts, not only that it passes.",
		whatToCheck(test, []*engine.TouchedFunction{fn("TestPut", 0, 5, true)}))
	assert.Equal(t, "These tests have logic of their own, `TestPut()` most of all (complexity 5). Check what they assert, not only that they pass.",
		whatToCheck(test, []*engine.TouchedFunction{fn("TestGet", 0, 4, true), fn("TestPut", 0, 5, true)}))
	assert.Equal(t, "This change adds 2 branches to `Put()` (complexity 3 → 5). For each new condition, check when it is taken and whether a test takes it. These lines also touch `Get()` (complexity 6) and `List()` (complexity 4).",
		whatToCheck(code, []*engine.TouchedFunction{fn("List", 4, 4, false), fn("Put", 3, 5, false), fn("Get", 0, 6, true)}), "one thing to check, the rest is named")
	assert.Empty(t, whatToCheck(code, nil))
}

func TestReviewKeepsTheFirstPlaces(t *testing.T) {
	p := readingPlan()
	review := p.Review(2)
	require.Len(t, review.Comments, 2)
	assert.Len(t, p.Review(0).Comments, 6, "ten by default; four spans at most for store.go, whose functions are not known here")
}

func TestAnnotations(t *testing.T) {
	p := readingPlan()
	p.Facts = []engine.Fact{{Kind: engine.FactCycle, Path: "core/store.go", Text: "introduces a cycle: 100% of store.go"}}
	p.File("core/store.go").Functions = []engine.TouchedFunction{{Name: "Store::Put", Line: 9, Before: 2, After: 6}}
	out := p.Annotations(0)
	assert.Equal(t, ""+
		"::notice file=core/store.go,line=10,endLine=14,title=Cribelo%3A read closely::This change adds 4 branches to Put() (complexity 2 → 6). For each new condition, check when it is taken and whether a test takes it. "+
		"The change touches it again on lines 30, 40-62, 70-71, 80-90 and 1 more.%0A%0A2 files depend on this file.%0A%0Aintroduces a cycle: 100%25 of store.go\n"+
		"::notice file=core/store_test.go,line=8,endLine=8,title=Cribelo%3A read closely::Check what this test asserts, not only that it passes.\n"+
		"::notice file=cmd/store.go,line=5,endLine=7,title=Cribelo%3A read closely::Cribelo counts these lines among those to read closely.\n", out)
	assert.Len(t, strings.Split(strings.TrimSpace(p.Annotations(1)), "\n"), 1)
}

func TestReviewWordsWhyThisFile(t *testing.T) {
	f := &engine.FileReport{Values: map[engine.Feature]float64{engine.FeatureUntested: 1}}
	assert.Equal(t, []string{"No test of this file is part of the change."}, whyThisFile(f, nil))
	f.Values[engine.FeatureAfferent] = 12
	f.Values[engine.FeatureFixes90d] = 2
	assert.Equal(t, []string{"12 files depend on this file, it needed 2 fixes in the last 90 days and no test of it is part of the change."}, whyThisFile(f, nil))
	assert.Empty(t, whyThisFile(&engine.FileReport{Test: true, Values: f.Values}, nil), "the signals are those of the code")
}
