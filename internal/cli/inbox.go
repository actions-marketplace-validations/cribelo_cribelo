package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	core "github.com/ast-metrics/ast-metrics/cribelo"
	"github.com/cribelo/cribelo/internal/render"
	"github.com/urfave/cli/v2"
)

// The inbox answers the question a plan cannot: of the pull requests that
// wait for me, which one now? Each is given its reading time, the same
// one its own plan would give, and the shortest come first: ten minutes
// spent there unblock someone today.
//
// The list comes from the command line of the forge (gh, or glab on
// GitLab), the code from the pull request refs of the remote. Each head is checked out in a temporary worktree and
// planned like a local change; the plan of a head already seen is kept
// under the git directory.

// pullRequest is an open pull request, in the words of gh; a merge request
// of GitLab is read into the same fields.
type pullRequest struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	HeadRefOid  string `json:"headRefOid"`
	BaseRefName string `json:"baseRefName"`
	IsDraft     bool   `json:"isDraft"`
	UpdatedAt   string `json:"updatedAt"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
}

const inboxCacheVersion = 1

// inboxCache keeps the measured pull requests by head and base commit.
type inboxCache struct {
	Version int                         `json:"version"`
	Entries map[string]render.InboxItem `json:"entries"`
}

func runInbox(c *cli.Context) error {
	format := c.String("format")
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("unknown format %q for inbox (text or json)", format)
	}
	repo, err := core.OpenRepo(c.String("dir"))
	if err != nil {
		return err
	}
	site, err := detectForge(repo.Root, c.String("forge"))
	if err != nil {
		return err
	}
	pulls, err := site.list(repo.Root, c.Int("limit"), c.Bool("all"))
	if err != nil {
		return err
	}
	if !c.Bool("drafts") {
		kept := pulls[:0]
		for _, p := range pulls {
			if !p.IsDraft {
				kept = append(kept, p)
			}
		}
		pulls = kept
	}

	inbox := render.Inbox{All: c.Bool("all"), Noun: site.noun(), Sign: site.sign()}
	if len(pulls) > 0 {
		progress := func(string) {}
		if format != "json" && c.App.ErrWriter != nil {
			progress = func(line string) { fmt.Fprintln(c.App.ErrWriter, line) }
		}
		inbox.Items, err = measurePulls(repo.Root, site, pulls, !c.Bool("no-clones"), progress)
		if err != nil {
			return err
		}
	}
	sort.SliceStable(inbox.Items, func(i, j int) bool {
		a, b := inbox.Items[i], inbox.Items[j]
		if (a.Error == "") != (b.Error == "") {
			return a.Error == ""
		}
		if a.Minutes != b.Minutes {
			return a.Minutes < b.Minutes
		}
		return a.Number < b.Number
	})
	if format == "json" {
		data, err := json.MarshalIndent(inbox.Items, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Print(inbox.Text(time.Now()))
	return nil
}

// fetchPulls brings the heads and the bases of pull requests into the
// repository, under refs/cribelo. It returns, by number, the pull
// requests whose head could not be fetched, with the reason.
func fetchPulls(root string, site forge, pulls []pullRequest) map[int]string {
	remote := site.Remote
	// The bases move: they are fetched every time. A head already here is
	// not fetched again, so that a second look at the inbox costs one
	// round trip. When a head is gone (a fork deleted with its branch),
	// the others are fetched one by one and that one says so in the list.
	var refspecs []string
	bases := map[string]bool{}
	for _, p := range pulls {
		if !bases[p.BaseRefName] {
			bases[p.BaseRefName] = true
			refspecs = append(refspecs, fmt.Sprintf("+refs/heads/%s:refs/cribelo/base/%s", p.BaseRefName, p.BaseRefName))
		}
	}
	var wanted []pullRequest
	for _, p := range pulls {
		if p.HeadRefOid == "" {
			continue
		}
		if _, err := gitIn(root, "cat-file", "-e", p.HeadRefOid+"^{commit}"); err != nil {
			wanted = append(wanted, p)
			refspecs = append(refspecs, fmt.Sprintf("+%s:refs/cribelo/pull/%d", site.headRef(p.Number), p.Number))
		}
	}
	unfetched := map[int]string{}
	if _, err := gitIn(root, append([]string{"fetch", "--quiet", "--no-tags", remote}, refspecs...)...); err != nil {
		for base := range bases {
			gitIn(root, "fetch", "--quiet", "--no-tags", remote, fmt.Sprintf("+refs/heads/%s:refs/cribelo/base/%s", base, base))
		}
		for _, p := range wanted {
			if _, err := gitIn(root, "fetch", "--quiet", "--no-tags", remote, fmt.Sprintf("+%s:refs/cribelo/pull/%d", site.headRef(p.Number), p.Number)); err != nil {
				unfetched[p.Number] = "its head is not on " + remote
			}
		}
	}
	return unfetched
}

// measurePulls plans every pull request. A pull request that cannot be
// measured keeps its place in the list with the reason.
func measurePulls(root string, site forge, pulls []pullRequest, clones bool, progress func(string)) ([]render.InboxItem, error) {
	unfetched := fetchPulls(root, site, pulls)

	cachePath := ""
	cache := inboxCache{Version: inboxCacheVersion, Entries: map[string]render.InboxItem{}}
	if common, err := gitIn(root, "rev-parse", "--path-format=absolute", "--git-common-dir"); err == nil {
		cachePath = filepath.Join(strings.TrimSpace(common), "cribelo", "inbox.json")
		if data, err := os.ReadFile(cachePath); err == nil {
			var stored inboxCache
			if json.Unmarshal(data, &stored) == nil && stored.Version == inboxCacheVersion && stored.Entries != nil {
				cache = stored
			}
		}
	}

	items := make([]render.InboxItem, 0, len(pulls))
	live := map[string]bool{}
	for i, p := range pulls {
		item := render.InboxItem{Number: p.Number, Title: p.Title, URL: p.URL, Author: p.Author.Login, Draft: p.IsDraft, UpdatedAt: p.UpdatedAt}
		if p.HeadRefOid == "" {
			item.Error = "no commit yet"
			items = append(items, item)
			continue
		}
		if reason, failed := unfetched[p.Number]; failed {
			item.Error = reason
			items = append(items, item)
			continue
		}
		baseSha, err := gitIn(root, "rev-parse", "refs/cribelo/base/"+p.BaseRefName)
		if err != nil {
			item.Error = "base branch not found"
			items = append(items, item)
			continue
		}
		key := fmt.Sprintf("%s..%s", strings.TrimSpace(baseSha), p.HeadRefOid)
		live[key] = true
		if cached, ok := cache.Entries[key]; ok {
			item.Minutes, item.Files, item.ToRead, item.Facts = cached.Minutes, cached.Files, cached.ToRead, cached.Facts
			items = append(items, item)
			continue
		}
		progress(fmt.Sprintf("reading %s%d (%d of %d)", site.sign(), p.Number, i+1, len(pulls)))
		plan, err := planAt(root, p.HeadRefOid, "refs/cribelo/base/"+p.BaseRefName, core.Options{Clones: clones, Coverage: "none"})
		if err != nil {
			item.Error = err.Error()
			items = append(items, item)
			continue
		}
		item.Minutes, item.Files, item.ToRead, item.Facts = plan.ChangeMinutes, plan.Changed, plan.LinesToRead, len(plan.Facts)
		cache.Entries[key] = item
		items = append(items, item)
	}
	// The plans of heads that moved on are dropped.
	for key := range cache.Entries {
		if !live[key] {
			delete(cache.Entries, key)
		}
	}
	if cachePath != "" {
		if data, err := json.Marshal(cache); err == nil && os.MkdirAll(filepath.Dir(cachePath), 0o755) == nil {
			os.WriteFile(cachePath, data, 0o644)
		}
	}
	return items, nil
}

// planAt plans the change a commit brings over a base, in a temporary
// worktree: the checkout of the user is not touched.
func planAt(root, head, base string, opts core.Options) (*core.Plan, error) {
	dir, err := os.MkdirTemp("", "cribelo-inbox-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	if _, err := gitIn(root, "worktree", "add", "--detach", dir, head); err != nil {
		return nil, err
	}
	defer func() {
		gitIn(root, "worktree", "remove", "--force", dir)
		gitIn(root, "worktree", "prune")
	}()
	repo, err := core.OpenRepo(dir)
	if err != nil {
		return nil, err
	}
	opts.Base = base
	return core.Run(repo, opts)
}

func gitIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
