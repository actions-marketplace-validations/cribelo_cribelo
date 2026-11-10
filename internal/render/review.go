package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	engine "github.com/ast-metrics/ast-metrics/cribelo"
)

// Marker ends every comment Cribelo posts on the lines of a pull request, so
// that the next run can find and remove those of the run before. The scripts
// that post the review look for the same string.
const Marker = "<!-- cribelo-review -->"

// Review is the body of a GitHub pull request review: one comment on each
// span of lines to read closely, in the files of the reading list, as the
// reviews endpoint of the REST API takes it.
type Review struct {
	// CommitID is the head commit the lines are numbered on. The script
	// that posts the review replaces it with the head of the pull request
	// when the run checked out a merge commit.
	CommitID string          `json:"commit_id,omitempty"`
	Event    string          `json:"event"`
	Body     string          `json:"body"`
	Comments []ReviewComment `json:"comments"`
}

// ReviewComment marks a span of lines of a file on the new side of the diff.
// A single line has no start.
type ReviewComment struct {
	Path      string `json:"path"`
	Body      string `json:"body"`
	Line      int    `json:"line"`
	Side      string `json:"side"`
	StartLine int    `json:"start_line,omitempty"`
	StartSide string `json:"start_side,omitempty"`
}

// Review builds the review of the plan: in each file of the reading list,
// the first spans to read closely, maxRanges at most as in the plan, each
// with the functions it falls in and how their complexity moved. The facts
// about a file go with its first comment.
func (p Report) Review() Review {
	review := Review{CommitID: p.HeadSha, Event: "COMMENT", Comments: []ReviewComment{}}
	files := 0
	for _, r := range p.rows() {
		f := r.file
		if len(f.Ranges) == 0 {
			continue
		}
		files++
		for i, span := range f.Ranges {
			if i == maxRanges {
				break
			}
			c := ReviewComment{Path: f.Path, Line: span.End, Side: "RIGHT", Body: spanComment(f, span, i == 0, p.Facts)}
			if span.End > span.Start {
				c.StartLine, c.StartSide = span.Start, "RIGHT"
			}
			review.Comments = append(review.Comments, c)
		}
	}
	n := len(review.Comments)
	switch {
	case n == 0:
		review.Body = "Cribelo found no lines to read closely in this change."
	default:
		review.Body = fmt.Sprintf("Cribelo marks %d %s to read closely in %d %s. The reading map is in the comment of this pull request.", n, engine.Plural(n, "place"), files, engine.Plural(files, "file"))
	}
	review.Body += "\n\n" + Marker
	return review
}

// spanComment words one span: what to do, the functions it falls in with
// their complexity, then, on the first span of the file, what the diff does
// not show about the file.
func spanComment(f *engine.FileReport, span engine.LineRange, first bool, facts []engine.Fact) string {
	var b strings.Builder
	b.WriteString("**Read closely**")
	if parts := functionsIn(f, span); len(parts) > 0 {
		b.WriteString(" · " + strings.Join(parts, ", "))
	}
	if first {
		if own := engine.FactsOf(facts, f.Path); len(own) > 0 {
			b.WriteString("\n\n**Worth knowing**")
			for _, fact := range own {
				b.WriteString("\n- " + fact.Markdown())
			}
		}
	}
	b.WriteString("\n\n" + Marker)
	return b.String()
}

// functionsIn names the functions a span falls in, with how their complexity
// moved: "`Parse()` complexity 3 → 6", "`Emit()` new, complexity 4". A
// function whose end is unknown holds the span when it starts the closest
// before it.
func functionsIn(f *engine.FileReport, span engine.LineRange) []string {
	var out []string
	var closest *engine.TouchedFunction
	for i := range f.Functions {
		fn := &f.Functions[i]
		if fn.End > 0 {
			if fn.Line <= span.End && span.Start <= fn.End {
				out = append(out, functionState(fn))
			}
			continue
		}
		if fn.Line <= span.Start && (closest == nil || fn.Line > closest.Line) {
			closest = fn
		}
	}
	if len(out) == 0 && closest != nil {
		out = append(out, functionState(closest))
	}
	return out
}

func functionState(fn *engine.TouchedFunction) string {
	name := code(engine.ShortName(fn.Name) + "()")
	switch {
	case fn.New:
		return fmt.Sprintf("%s new, complexity %d", name, fn.After)
	case fn.Before != fn.After:
		return fmt.Sprintf("%s complexity %d → %d", name, fn.Before, fn.After)
	}
	return fmt.Sprintf("%s complexity %d", name, fn.After)
}

// JSON renders the review as the reviews endpoint takes it. The marker is
// kept as it is, not escaped, so that the file reads like the comments.
func (r Review) JSON() (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return "", err
	}
	return b.String(), nil
}
