package cli

import (
	"testing"

	core "github.com/ast-metrics/ast-metrics/cribelo"
	"github.com/stretchr/testify/assert"
)

func TestGlanceableKeepsWhatAReviewerNeedNotOpen(t *testing.T) {
	plan := &core.Plan{Skipped: []core.Skipped{
		{Path: "moved.go", Kind: "code", Reason: "moved code only", Mechanical: true},
		{Path: "small.go", Kind: "code", Reason: "no measurable change"},
		{Path: "ranked.go", Kind: "code", Reason: "ranked lower"},
		{Path: "old_name.go", Kind: "renamed", Reason: "renamed, unchanged"},
		{Path: "go.sum", Kind: "generated", Reason: "generated"},
		{Path: "a_test.go", Kind: "tests", Reason: "tests only"},
		{Path: "README.md", Kind: "not analyzed", Reason: "not analyzed (docs, config, other languages)"},
		{Path: "gone.go", Kind: "deleted", Reason: "deleted"},
	}}
	var paths []string
	for _, s := range glanceable(plan) {
		paths = append(paths, s.Path)
	}
	assert.Equal(t, []string{"moved.go", "old_name.go", "go.sum"}, paths)
}
