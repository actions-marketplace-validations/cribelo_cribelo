package render

import (
	"testing"

	engine "github.com/ast-metrics/ast-metrics/cribelo"
	"github.com/stretchr/testify/assert"
)

func TestBacktestText(t *testing.T) {
	SetColor(false)
	m := engine.Metrics{Changes: 4, Files: 20, FixedFiles: 5, Covered: 3, Coverage: 0.6, LinesShare: 0.5, RuleCoverage: 0.8, RuleShare: 0.7, RuleRead: 10, BaselineCover: 0.4, BaselineShare: 0.3, ToReadShare: 0.9, ToReadTopShare: 0.45, PathOrderCover: 0.2, PathOrderShare: 0.1}
	b := &engine.Backtest{Ref: "main", Top: 3, Landings: 4, Label: engine.LabelInduced, From: 1704067200, To: 1706745600,
		Changes: make([]engine.ChangeEval, 4), LeftOut: map[string]int{"too recent to know if a fix followed": 2}, Generic: m}
	text := Backtest(b)
	assert.Contains(t, text, "cribelo backtest · 4 landings on main replayed")
	assert.Contains(t, text, "4 changes qualified, 2024-01-01 → 2024-02-01: 20 code files, 5 of them (25%) wrote lines a later fix repaired.")
	assert.Contains(t, text, "generic weights, all qualified changes\n  Setting aside moves, extractions, renames, formatting and imports leaves 90% of the changed lines to read.")
	assert.Contains(t, text, "Reading every high file, at least 3 (2.5 files a change): 80%, for 70% of the lines.")
	assert.Contains(t, text, "Left out: too recent to know if a fix followed 2")
	assert.NotContains(t, text, "calibrated weights")

	b.Calibrated, b.GenericHeldOut = &m, &m
	assert.Contains(t, Backtest(b), "held-out check, the 4 most recent changes\n  generic weights     top-3 covers 60% of those files for 50% of the lines; every high file: 80% for 70%\n  calibrated weights  top-3")

	b.Label = engine.LabelFixed
	assert.Contains(t, Backtest(b), "later touched by a fix")
	assert.Contains(t, Backtest(&engine.Backtest{Ref: "main"}), "No landing qualified.")

	assert.Equal(t, "a 1 · b 22 · c 3", countsLine(map[string]int{"c": 3, "a": 1, "b": 22}))
	assert.Equal(t, "", countsLine(nil))
	assert.Equal(t, "2024-01-01", day(1704067200))
}
