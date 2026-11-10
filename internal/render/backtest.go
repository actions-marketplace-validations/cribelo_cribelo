package render

import (
	"fmt"
	"sort"
	"strings"
	"time"

	engine "github.com/ast-metrics/ast-metrics/cribelo"
)

// Backtest renders a backtest for a terminal: how many landings were
// replayed, how well each way of picking the files to read covered those a
// later fix proved worth reading, and, after a calibration, the held-out
// comparison of the two models.
func Backtest(b *engine.Backtest) string {
	var s strings.Builder
	fmt.Fprintf(&s, "%s %s\n\n", styleTitle.Render("cribelo backtest"), styleDim.Render(fmt.Sprintf("· %d landings on %s replayed", b.Landings, b.Ref)))
	if len(b.Changes) == 0 {
		s.WriteString("No landing qualified.\n")
	} else {
		share := float64(b.Generic.FixedFiles) / float64(max(b.Generic.Files, 1))
		fmt.Fprintf(&s, "%d changes qualified, %s → %s: %d code files, %d of them (%s) %s.\n\n", len(b.Changes), day(b.From), day(b.To), b.Generic.Files, b.Generic.FixedFiles, engine.Percent(share), positiveWording(b))
		s.WriteString(metricsLines(b.Generic, b.Top, "generic weights, all qualified changes"))
		if b.Calibrated != nil && b.GenericHeldOut != nil {
			fmt.Fprintf(&s, "\n%s\n", styleTitle.Render(fmt.Sprintf("held-out check, the %d most recent changes", b.Calibrated.Changes)))
			fmt.Fprintf(&s, "  generic weights     top-%d covers %s of those files for %s of the lines; every high file: %s for %s\n", b.Top, engine.Percent(b.GenericHeldOut.Coverage), engine.Percent(b.GenericHeldOut.LinesShare), engine.Percent(b.GenericHeldOut.RuleCoverage), engine.Percent(b.GenericHeldOut.RuleShare))
			fmt.Fprintf(&s, "  calibrated weights  top-%d covers %s of those files for %s of the lines; every high file: %s for %s\n", b.Top, engine.Percent(b.Calibrated.Coverage), engine.Percent(b.Calibrated.LinesShare), engine.Percent(b.Calibrated.RuleCoverage), engine.Percent(b.Calibrated.RuleShare))
			fmt.Fprintf(&s, "  %d largest files     cover %s of those files for %s of the lines\n", b.Top, engine.Percent(b.Calibrated.BaselineCover), engine.Percent(b.Calibrated.BaselineShare))
		}
	}
	if len(b.LeftOut) > 0 {
		s.WriteString("\n" + styleDim.Render("Left out: "+countsLine(b.LeftOut)) + "\n")
	}
	fixWords := "a commit whose message says fix, bug, revert, regression, correction, corrigé... (English, French, Spanish, Portuguese, German; add yours with fix_pattern in .cribelo.yaml)"
	if b.Label == engine.LabelFixed {
		s.WriteString(styleDim.Render("A file counts when "+fixWords+" touches it within 90 days. Hub signal from today's graph; clones not replayed.") + "\n")
	} else {
		s.WriteString(styleDim.Render("A file counts when "+fixWords+", within 90 days, changes or deletes lines that git blame attributes to this change in that file (SZZ). Hub signal from today's graph; clones not replayed.") + "\n")
	}
	return s.String()
}

func positiveWording(b *engine.Backtest) string {
	if b.Label == engine.LabelFixed {
		return "later touched by a fix"
	}
	return "wrote lines a later fix repaired"
}

func metricsLines(m engine.Metrics, top int, label string) string {
	var s strings.Builder
	fmt.Fprintf(&s, "%s\n", styleTitle.Render(label))
	fmt.Fprintf(&s, "  Setting aside moves, extractions, renames, formatting and imports leaves %s of the changed lines to read.\n", engine.Percent(m.ToReadShare))
	fmt.Fprintf(&s, "  Reading the top-%d files covered %s of those files, for %s of the changed lines (%s once classified).\n", top, engine.Percent(m.Coverage), engine.Percent(m.LinesShare), engine.Percent(m.ToReadTopShare))
	fmt.Fprintf(&s, "  Reading the first %d in path order, as a pull request lists them: %s, for %s of the lines.\n", top, engine.Percent(m.PathOrderCover), engine.Percent(m.PathOrderShare))
	fmt.Fprintf(&s, "  Reading the %d largest files instead: %s, for %s of the lines.\n", top, engine.Percent(m.BaselineCover), engine.Percent(m.BaselineShare))
	perChange := float64(m.RuleRead) / float64(max(m.Changes, 1))
	fmt.Fprintf(&s, "  Reading every high file, at least %d (%.1f files a change): %s, for %s of the lines.\n", top, perChange, engine.Percent(m.RuleCoverage), engine.Percent(m.RuleShare))
	return s.String()
}

func day(t int64) string { return time.Unix(t, 0).UTC().Format("2006-01-02") }

// countsLine words counts by name, names sorted: "a 1 · b 22".
func countsLine(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, counts[k]))
	}
	return strings.Join(parts, " · ")
}
