package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInboxText(t *testing.T) {
	SetColor(false)
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	in := Inbox{Items: []InboxItem{
		{Number: 212, Title: "fix(engine): read a Rust item", Author: "ada", UpdatedAt: "2026-11-11T08:00:00Z", Minutes: 4},
		{Number: 98, Title: "feat: a title that goes on and on, far beyond what a line of a terminal can hold", Author: "grace", UpdatedAt: "2026-11-02T08:00:00Z", Minutes: 77},
		{Number: 7, Title: "wip", Author: "linus", UpdatedAt: "2026-08-01T08:00:00Z", Draft: true, Error: "base branch not found"},
	}}
	text := in.Text(now)
	assert.Contains(t, text, "cribelo inbox · 3 pull requests wait for your review, about 1 h 21 of reading\n")
	assert.Contains(t, text, "  #212   4 min  fix(engine): read a Rust item                                    ada    today\n")
	assert.Contains(t, text, "   #98  1 h 17  feat: a title that goes on and on, far beyond what a line of a…  grace    9 d\n")
	assert.Contains(t, text, "    #7       ?  wip                                                              linus   3 mo  not read: base branch not found, draft\n")

	assert.Equal(t, "cribelo inbox · nothing waits for your review\n", Inbox{}.Text(now))
	assert.Equal(t, "cribelo inbox · no open pull request\n", Inbox{All: true}.Text(now))
	assert.Contains(t, Inbox{All: true, Items: in.Items[:1]}.Text(now), "1 open pull request, about 4 min of reading")

	// The words of GitLab.
	in.Noun, in.Sign = "merge request", "!"
	text = in.Text(now)
	assert.Contains(t, text, "cribelo inbox · 3 merge requests wait for your review, about 1 h 21 of reading\n")
	assert.Contains(t, text, "  !212   4 min  fix(engine): read a Rust item")
	assert.Equal(t, "cribelo inbox · no open merge request\n", Inbox{All: true, Noun: "merge request"}.Text(now))
}
