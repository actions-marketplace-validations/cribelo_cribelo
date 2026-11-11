package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	engine "github.com/ast-metrics/ast-metrics/cribelo"
)

// Marker ends every comment Cribelo posts on the lines of a pull request, so
// that the next run can find and remove those of the run before. The scripts
// that post the review look for the same string.
const Marker = "<!-- cribelo-review -->"

// DefaultMaxComments is the number of places marked on a diff when nothing
// else is asked: enough to lead the reader, few enough to leave the diff
// readable.
const DefaultMaxComments = 10

// maxAnnotations is what GitHub keeps of the notices one step of a job
// writes; the ones after are dropped without a word.
const maxAnnotations = 10

// Review is the body of a GitHub pull request review: one comment on each
// span of lines to read closely, in the files of the reading list, as the
// reviews endpoint of the REST API takes it.
type Review struct {
	// CommitID is the head commit the lines are numbered on. The script
	// that posts the review replaces it with the head of the pull request
	// when the run checked out a merge commit.
	CommitID string          `json:"commit_id,omitempty"`
	Event    string          `json:"event"`
	Body     string          `json:"body,omitempty"`
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

// place is one span to read closely with what to tell its reader: what to
// check in it, then, on the first span of a file, why this file. Each
// paragraph is Markdown.
type place struct {
	path       string
	span       engine.LineRange
	paragraphs []string
	// functions are those the comment tells; again the later spans of the
	// file that fall in the same ones, named here rather than marked.
	functions int
	again     []engine.LineRange
}

// places lists the places to read closely in reading order, max at most
// (DefaultMaxComments when max is not positive), and counts all of them. A
// span in a function already told on an earlier span of the file is not a
// place of its own: the first comment names its lines. A file gives
// maxRanges places at most, as it shows that many spans in the plan.
func (p Report) places(max int) (out []place, total int) {
	if max <= 0 {
		max = DefaultMaxComments
	}
	for _, r := range p.rows() {
		f := r.file
		told := map[string]int{}
		start := len(out)
		for _, span := range f.Ranges {
			fns := functionsIn(f, span)
			if at, done := toldAt(fns, told); done {
				out[at].again = append(out[at].again, span)
				continue
			}
			if len(out)-start == maxRanges {
				continue
			}
			var fresh []*engine.TouchedFunction
			for _, fn := range fns {
				if _, done := told[fn.Name]; !done {
					fresh = append(fresh, fn)
					told[fn.Name] = len(out)
				}
			}
			paragraphs := []string{whatToCheck(f, fresh)}
			if len(out) == start {
				paragraphs = append(paragraphs, whyThisFile(f, p.Facts)...)
			}
			out = append(out, place{path: f.Path, span: span, paragraphs: paragraphs, functions: len(fresh)})
		}
	}
	total = len(out)
	if total > max {
		out = out[:max]
	}
	for i := range out {
		if again := out[i].again; len(again) > 0 {
			out[i].paragraphs[0] += fmt.Sprintf(" The change touches %s again on %s.", pick(out[i].functions, "it", "them"), lines(again))
		}
	}
	return out, total
}

// toldAt says whether every function of a span was told by an earlier
// place of the file, and gives the place that told the first one.
func toldAt(fns []*engine.TouchedFunction, told map[string]int) (int, bool) {
	if len(fns) == 0 {
		return 0, false
	}
	for _, fn := range fns {
		if _, done := told[fn.Name]; !done {
			return 0, false
		}
	}
	return told[fns[0].Name], true
}

// lines words spans for a sentence: "line 141", "lines 122-127, 141 and
// 152". Past four spans the rest is counted.
func lines(spans []engine.LineRange) string {
	var parts []string
	for i, r := range spans {
		if i == maxRanges {
			parts = append(parts, fmt.Sprintf("%d more", len(spans)-i))
			break
		}
		if r.End > r.Start {
			parts = append(parts, fmt.Sprintf("%d-%d", r.Start, r.End))
		} else {
			parts = append(parts, fmt.Sprintf("%d", r.Start))
		}
	}
	if len(spans) == 1 && spans[0].End <= spans[0].Start {
		return "line " + parts[0]
	}
	return "lines " + joinAnd(parts)
}

// Review builds the review of the plan: a comment on each of the first
// places to read closely, max at most, that says what to check there and
// why. What the diff does not show about a file goes with its first comment.
func (p Report) Review(max int) Review {
	review := Review{CommitID: p.HeadSha, Event: "COMMENT", Comments: []ReviewComment{}}
	places, _ := p.places(max)
	for _, pl := range places {
		body := "**Read closely.**"
		if pl.paragraphs[0] != "" {
			body += " " + pl.paragraphs[0]
		}
		for _, more := range pl.paragraphs[1:] {
			body += "\n\n" + more
		}
		c := ReviewComment{Path: pl.path, Line: pl.span.End, Side: "RIGHT", Body: body + "\n\n" + Marker}
		if pl.span.End > pl.span.Start {
			c.StartLine, c.StartSide = pl.span.Start, "RIGHT"
		}
		review.Comments = append(review.Comments, c)
	}
	// A review with comments carries no text of its own: GitHub keeps the
	// text of every review in the conversation, one more at each push, when
	// the comments of the run before can be removed.
	if len(places) == 0 {
		review.Body = "Cribelo found no lines to read closely in this change.\n\n" + Marker
	}
	return review
}

// Annotations words the same places as notices for a GitHub Actions job
// log, one workflow command a line. GitHub shows them on the lines of the
// diff, and a job needs no right to write them: this is what marks the
// lines of a pull request whose token cannot post a review. They are plain
// text.
func (p Report) Annotations(max int) string {
	if max <= 0 || max > maxAnnotations {
		max = maxAnnotations
	}
	places, _ := p.places(max)
	var b strings.Builder
	for _, pl := range places {
		text := strings.Join(pl.paragraphs, "\n\n")
		if pl.paragraphs[0] == "" {
			text = strings.TrimPrefix(text, "\n\n")
		}
		if text == "" {
			text = "Cribelo counts these lines among those to read closely."
		}
		fmt.Fprintf(&b, "::notice file=%s,line=%d,endLine=%d,title=%s::%s\n",
			commandProperty(pl.path), pl.span.Start, pl.span.End, commandProperty("Cribelo: read closely"), commandData(plainText(text)))
	}
	return b.String()
}

// plainText drops the Markdown of a comment: the marks around identifiers,
// the bold, the bullets.
func plainText(s string) string {
	s = strings.NewReplacer("`", "", "**", "").Replace(s)
	return strings.ReplaceAll(s, "\n- ", "\n")
}

// commandData escapes the message of a workflow command, commandProperty
// one of its properties.
func commandData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

func commandProperty(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}

// whatToCheck words what a reader looks for in one span, from the functions
// it falls in that no earlier comment told: the branches a function gained,
// the code that is new, the lines changed inside branches already there. A
// span that mixes them gets the first of these, and the names of the other
// functions. The text is empty when nothing is known about the span.
func whatToCheck(f *engine.FileReport, fns []*engine.TouchedFunction) string {
	if len(fns) == 0 {
		if f.Test {
			return "Check what this test asserts, not only that it passes."
		}
		return ""
	}
	// The functions with the most branches are named first.
	fresh := append([]*engine.TouchedFunction(nil), fns...)
	sort.SliceStable(fresh, func(i, j int) bool { return fresh[i].After > fresh[j].After })

	if f.Test {
		if len(fresh) == 1 {
			return fmt.Sprintf("This test has logic of its own: %s has complexity %d. Check what it asserts, not only that it passes.", name(fresh[0]), fresh[0].After)
		}
		return fmt.Sprintf("These tests have logic of their own, %s most of all (complexity %d). Check what they assert, not only that they pass.", name(fresh[0]), fresh[0].After)
	}

	var grown, added, shrunk, same []*engine.TouchedFunction
	for _, fn := range fresh {
		switch {
		case fn.New:
			added = append(added, fn)
		case fn.After > fn.Before:
			grown = append(grown, fn)
		case fn.After < fn.Before:
			shrunk = append(shrunk, fn)
		default:
			same = append(same, fn)
		}
	}
	var sentences []string
	switch {
	case len(grown) == 1:
		fn := grown[0]
		gain := fn.After - fn.Before
		already := ""
		if fn.Before >= 10 {
			already = ", which already had many"
		}
		check := "Check when the new condition is taken and whether a test takes it."
		if gain > 1 {
			check = "For each new condition, check when it is taken and whether a test takes it."
		}
		sentences = append(sentences, fmt.Sprintf("This change adds %d %s to %s%s (complexity %d → %d). %s",
			gain, pick(gain, "branch", "branches"), name(fn), already, fn.Before, fn.After, check))
	case len(grown) > 1:
		sentences = append(sentences, fmt.Sprintf("This change adds branches to %s. For each new condition, check when it is taken and whether a test takes it.", states(grown)))
	}
	switch {
	case len(added) > 2:
		sentences = append(sentences, fmt.Sprintf("New code, nothing to compare it with. %s and %s hold most of the branching (complexity %d and %d): start there.",
			name(added[0]), name(added[1]), added[0].After, added[1].After))
	case len(added) == 2:
		sentences = append(sentences, fmt.Sprintf("%s and %s are new (complexity %d and %d), nothing to compare them with. Check the cases they handle, and the ones they leave out.",
			name(added[0]), name(added[1]), added[0].After, added[1].After))
	case len(added) == 1:
		sentences = append(sentences, fmt.Sprintf("%s is new (complexity %d), nothing to compare it with. Check the cases it handles, and the ones it leaves out.", name(added[0]), added[0].After))
	}
	if len(shrunk) > 0 {
		sentences = append(sentences, fmt.Sprintf("This change removes branches from %s. Check that the cases they handled cannot happen any more.", states(shrunk)))
	}
	if len(same) > 0 {
		sentences = append(sentences, fmt.Sprintf("No new branch in %s, but the changed lines sit inside existing ones. Check that they hold in every case, not only the one the change was written for.", states(same)))
	}
	// One thing to check a comment: the first kind found leads, the
	// functions of the other kinds are only named.
	var rest []*engine.TouchedFunction
	lead := true
	for _, group := range [][]*engine.TouchedFunction{grown, added, shrunk, same} {
		if len(group) == 0 {
			continue
		}
		if !lead {
			rest = append(rest, group...)
		}
		lead = false
	}
	if len(rest) > 0 {
		return sentences[0] + " These lines also touch " + states(rest) + "."
	}
	return sentences[0]
}

// whyThisFile words what the diff does not show about a file, for its first
// comment: who depends on it and how often it had to be fixed, in one
// sentence, then the facts the engine states about it, one line each.
func whyThisFile(f *engine.FileReport, facts []engine.Fact) []string {
	var out []string
	if !f.Test {
		var parts []string
		if users := int(f.Values[engine.FeatureAfferent]); users > 1 {
			parts = append(parts, fmt.Sprintf("%d files depend on this file", users))
		}
		if fixes := int(f.Values[engine.FeatureFixes90d]); fixes > 0 {
			subject := "it"
			if len(parts) == 0 {
				subject = "this file"
			}
			parts = append(parts, fmt.Sprintf("%s needed %d %s in the last 90 days", subject, fixes, pick(fixes, "fix", "fixes")))
		}
		if f.Values[engine.FeatureUntested] > 0 {
			subject := "it"
			if len(parts) == 0 {
				subject = "this file"
			}
			parts = append(parts, "no test of "+subject+" is part of the change")
		}
		if len(parts) > 0 {
			sentence := joinAnd(parts) + "."
			out = append(out, strings.ToUpper(sentence[:1])+sentence[1:])
		}
	}
	if own := engine.FactsOf(facts, f.Path); len(own) > 0 {
		lines := make([]string, len(own))
		for i, fact := range own {
			lines[i] = "- " + fact.Markdown()
		}
		out = append(out, strings.Join(lines, "\n"))
	}
	return out
}

// functionsIn lists the functions a span falls in, in the order of the
// file. A function whose end is unknown holds the span when it starts the
// closest before it.
func functionsIn(f *engine.FileReport, span engine.LineRange) []*engine.TouchedFunction {
	var out []*engine.TouchedFunction
	var closest *engine.TouchedFunction
	for i := range f.Functions {
		fn := &f.Functions[i]
		if fn.End > 0 {
			if fn.Line <= span.End && span.Start <= fn.End {
				out = append(out, fn)
			}
			continue
		}
		if fn.Line <= span.Start && (closest == nil || fn.Line > closest.Line) {
			closest = fn
		}
	}
	if len(out) == 0 && closest != nil {
		out = append(out, closest)
	}
	return out
}

func name(fn *engine.TouchedFunction) string { return code(engine.ShortName(fn.Name) + "()") }

// names words a list of functions: "`A()`", "`A()` and `B()`", "`A()`,
// `B()` and `C()`".
func names(fns []*engine.TouchedFunction) string {
	parts := make([]string, len(fns))
	for i, fn := range fns {
		parts[i] = name(fn)
	}
	return joinAnd(parts)
}

// states words the same list with how the complexity of each one moved:
// "`Parse()` (complexity 3 → 6) and `Emit()` (complexity 4)". Past three
// functions the rest is counted.
func states(fns []*engine.TouchedFunction) string {
	var parts []string
	for i, fn := range fns {
		if i == 3 {
			rest := len(fns) - i
			parts = append(parts, fmt.Sprintf("%d %s", rest, pick(rest, "other", "others")))
			break
		}
		if fn.New || fn.Before == fn.After {
			parts = append(parts, fmt.Sprintf("%s (complexity %d)", name(fn), fn.After))
		} else {
			parts = append(parts, fmt.Sprintf("%s (complexity %d → %d)", name(fn), fn.Before, fn.After))
		}
	}
	return joinAnd(parts)
}

func joinAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

func pick(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
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
