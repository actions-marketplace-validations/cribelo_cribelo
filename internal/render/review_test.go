package render

import (
	"encoding/json"
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
	review := p.Review()

	assert.Equal(t, "fedcba9876", review.CommitID)
	assert.Equal(t, "COMMENT", review.Event)
	assert.Equal(t, "Cribelo marks 6 places to read closely in 3 files. The reading map is in the comment of this pull request.\n\n"+Marker, review.Body)
	require.Len(t, review.Comments, 6, "four spans at most for store.go, then its test, then cmd/store.go; handler.go is only glanced at")

	first := review.Comments[0]
	assert.Equal(t, ReviewComment{Path: "core/store.go", Line: 14, StartLine: 10, Side: "RIGHT", StartSide: "RIGHT",
		Body: "**Read closely** · `Put()` complexity 2 → 6\n\n**Worth knowing**\n- introduces a cycle: store.go → api.go → store.go\n\n" + Marker}, first)
	assert.Equal(t, ReviewComment{Path: "core/store.go", Line: 30, Side: "RIGHT", Body: "**Read closely** · `Get()` new, complexity 3\n\n" + Marker}, review.Comments[1], "a lone line has no start, the facts are told once")
	assert.Equal(t, "**Read closely** · `List()` complexity 4\n\n"+Marker, review.Comments[2].Body, "a function without a known end holds the spans after its start")
	assert.Equal(t, 71, review.Comments[3].Line)
	assert.Equal(t, "core/store_test.go", review.Comments[4].Path)
	assert.Equal(t, "**Read closely**\n\n"+Marker, review.Comments[4].Body, "no function known, no words about it")
	assert.Equal(t, ReviewComment{Path: "cmd/store.go", Line: 7, StartLine: 5, Side: "RIGHT", StartSide: "RIGHT", Body: "**Read closely**\n\n" + Marker}, review.Comments[5])

	out, err := review.JSON()
	require.NoError(t, err)
	var back map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &back))
	assert.Equal(t, "RIGHT", back["comments"].([]any)[0].(map[string]any)["side"])
	assert.NotContains(t, out, `"start_line": 0`, "a single line carries no start")
	assert.Contains(t, out, Marker, "the marker is not escaped")
}

func TestReviewNothingToRead(t *testing.T) {
	review := report(&engine.Plan{Changed: 2}).Review()
	assert.Empty(t, review.Comments)
	assert.Contains(t, review.Body, "no lines to read closely")
	out, err := review.JSON()
	require.NoError(t, err)
	assert.Contains(t, out, `"comments": []`, "an empty list, so that a script can count it")
}
