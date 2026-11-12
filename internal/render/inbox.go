package render

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	engine "github.com/ast-metrics/ast-metrics/cribelo"
)

// InboxItem is an open pull request with what its plan measures.
type InboxItem struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	URL       string `json:"url,omitempty"`
	Author    string `json:"author,omitempty"`
	Draft     bool   `json:"draft,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	// Minutes is the reading time of the whole change; ToRead the lines
	// to read closely.
	Minutes int `json:"minutes"`
	Files   int `json:"files"`
	ToRead  int `json:"lines_to_read"`
	Facts   int `json:"facts"`
	// Error says why the pull request could not be measured.
	Error string `json:"error,omitempty"`
}

// Inbox is the list of pull requests to review, the shortest first.
type Inbox struct {
	Items []InboxItem
	// All tells that every open pull request is listed, not only those
	// that wait for the reader.
	All bool
	// Noun and Sign are the words of the forge: "merge request" and "!"
	// on GitLab. Empty, they are those of GitHub.
	Noun string
	Sign string
}

// inboxTitle is the width a title is cut at.
const inboxTitle = 64

// Text renders the inbox for a terminal.
func (in Inbox) Text(now time.Time) string {
	var b strings.Builder
	noun, sign := in.Noun, in.Sign
	if noun == "" {
		noun = "pull request"
	}
	if sign == "" {
		sign = "#"
	}
	total, measured := 0, 0
	for _, item := range in.Items {
		if item.Error == "" {
			total += item.Minutes
			measured++
		}
	}
	var lead string
	switch {
	case len(in.Items) == 0 && in.All:
		lead = "no open " + noun
	case len(in.Items) == 0:
		lead = "nothing waits for your review"
	case in.All:
		lead = fmt.Sprintf("%d open %s", len(in.Items), engine.Plural(len(in.Items), noun))
	case len(in.Items) == 1:
		lead = "1 " + noun + " waits for your review"
	default:
		lead = fmt.Sprintf("%d %ss wait for your review", len(in.Items), noun)
	}
	if measured > 0 {
		lead += ", about " + engine.Duration(total) + " of reading"
	}
	fmt.Fprintf(&b, "%s %s\n", styleTitle.Render("cribelo inbox"), styleDim.Render("· "+lead))
	if len(in.Items) == 0 {
		return b.String()
	}
	b.WriteString("\n")

	numberWidth, timeWidth, titleWidth, authorWidth, ageWidth := 0, 0, 0, 0, 0
	titles, ages, times := make([]string, len(in.Items)), make([]string, len(in.Items)), make([]string, len(in.Items))
	for i, item := range in.Items {
		titles[i] = truncate(item.Title, inboxTitle)
		ages[i] = age(item.UpdatedAt, now)
		times[i] = engine.Duration(item.Minutes)
		switch {
		case item.Error != "":
			times[i] = "?"
		case item.Minutes == 0:
			// No file of a parsed language: nothing to time.
			times[i] = "no code"
		}
		numberWidth = max(numberWidth, len(fmt.Sprintf("%s%d", sign, item.Number)))
		timeWidth = max(timeWidth, len(times[i]))
		titleWidth = max(titleWidth, width(titles[i]))
		authorWidth = max(authorWidth, width(item.Author))
		ageWidth = max(ageWidth, len(ages[i]))
	}
	for i, item := range in.Items {
		line := fmt.Sprintf("  %*s  %*s  %s%s  %s%s  %s%s",
			numberWidth, fmt.Sprintf("%s%d", sign, item.Number), timeWidth, times[i],
			titles[i], strings.Repeat(" ", titleWidth-width(titles[i])),
			styleDim.Render(item.Author), strings.Repeat(" ", authorWidth-width(item.Author)),
			strings.Repeat(" ", ageWidth-len(ages[i])), styleDim.Render(ages[i]))
		var notes []string
		if item.Error != "" {
			notes = append(notes, "not read: "+item.Error)
		}
		if item.Draft {
			notes = append(notes, "draft")
		}
		if len(notes) > 0 {
			line += "  " + styleOrange.Render(strings.Join(notes, ", "))
		}
		b.WriteString(strings.TrimRight(line, " ") + "\n")
	}
	return b.String()
}

// truncate cuts a text at n characters, the last one an ellipsis.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return strings.TrimRight(string(runes[:n-1]), " ") + "…"
}

// age words how long ago a pull request moved: "today", "3 d", "2 w",
// "5 mo". Empty when the date cannot be read.
func age(updated string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, updated)
	if err != nil {
		return ""
	}
	days := int(now.Sub(t).Hours() / 24)
	switch {
	case days < 1:
		return "today"
	case days < 14:
		return fmt.Sprintf("%d d", days)
	case days < 60:
		return fmt.Sprintf("%d w", days/7)
	default:
		return fmt.Sprintf("%d mo", days/30)
	}
}
