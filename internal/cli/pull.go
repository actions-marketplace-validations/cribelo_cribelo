package cli

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	core "github.com/ast-metrics/ast-metrics/cribelo"
	"github.com/cribelo/cribelo/internal/render"
	"github.com/urfave/cli/v2"
)

// A reviewer goes from the inbox to a pull request without leaving what is
// checked out: `cribelo pr 212` fetches the head, plans it in a temporary
// worktree and prints the plan, its line ranges pointing at the diff on
// GitHub or at the file on GitLab. With --viewed, the files the plan sets aside whole are ticked as
// viewed for that reviewer: the Files tab opens on what is left to read.

// markViewed is the GraphQL mutation behind the "Viewed" checkbox.
const markViewed = `mutation($pr: ID!, $path: String!) { markFileAsViewed(input: {pullRequestId: $pr, path: $path}) { clientMutationId } }`

func gh(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = root
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %s", args[0]+" "+args[1], strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func runPull(c *cli.Context) error {
	// The flags are read after the number too: `cribelo pr 212 --viewed`
	// is how one types it.
	viewed, dryRun := c.Bool("viewed"), c.Bool("dry-run")
	var positional []string
	for _, arg := range c.Args().Slice() {
		switch arg {
		case "--viewed":
			viewed = true
		case "--dry-run":
			dryRun = true
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) != 1 {
		return errors.New("usage: cribelo pr <number> [--viewed] [--dry-run]")
	}
	number, err := strconv.Atoi(strings.TrimLeft(positional[0], "#!"))
	if err != nil {
		return errors.New("usage: cribelo pr <number> [--viewed] [--dry-run]")
	}
	repo, err := core.OpenRepo(c.String("dir"))
	if err != nil {
		return err
	}
	site, err := detectForge(repo.Root, c.String("forge"))
	if err != nil {
		return err
	}
	if viewed && site.Kind != github {
		return errors.New("--viewed works on GitHub only")
	}
	pull, id, err := site.view(repo.Root, number)
	if err != nil {
		return err
	}
	if pull.HeadRefOid == "" {
		return fmt.Errorf("%s%d has no commit yet", site.sign(), number)
	}
	if reason, failed := fetchPulls(repo.Root, site, []pullRequest{pull})[number]; failed {
		return fmt.Errorf("%s%d: %s", site.sign(), number, reason)
	}
	plan, err := planAt(repo.Root, pull.HeadRefOid, "refs/cribelo/base/"+pull.BaseRefName, planOptions(c, core.Options{}))
	if err != nil {
		return err
	}
	// The plan names the base by the branch, not by the ref it was fetched under.
	plan.Base = pull.BaseRefName
	report := render.Report{Plan: plan}
	if links, err := render.PullRequestLinks(pull.URL, plan.HeadSha); err == nil {
		report.Links = links
	}
	if err := printPlan(report, c.String("format"), c.Int("max-comments")); err != nil {
		return err
	}
	if viewed {
		if err := markGlanceable(repo.Root, id, number, plan, dryRun); err != nil {
			return err
		}
	}
	if report.Gate == "failed" {
		return errGateFailed
	}
	return nil
}

// glanceable lists the files of a plan a reviewer can tick without opening.
func glanceable(plan *core.Plan) []core.Skipped {
	var out []core.Skipped
	for _, s := range plan.Skipped {
		if s.Glanceable() {
			out = append(out, s)
		}
	}
	return out
}

// markGlanceable ticks the "Viewed" box of the files the plan sets aside
// whole. GitHub unticks a file by itself when a later push changes it.
func markGlanceable(root, id string, number int, plan *core.Plan, dryRun bool) error {
	files := glanceable(plan)
	fmt.Println()
	if len(files) == 0 {
		fmt.Printf("Viewed: nothing to mark on #%d, every changed file has something to read.\n", number)
		return nil
	}
	if dryRun {
		fmt.Printf("Viewed: would mark %d %s on #%d\n", len(files), core.Plural(len(files), "file"), number)
	} else {
		for _, f := range files {
			if _, err := gh(root, "api", "graphql", "-f", "query="+markViewed, "-f", "pr="+id, "-f", "path="+f.Path); err != nil {
				return fmt.Errorf("marking %s as viewed: %w", f.Path, err)
			}
		}
		fmt.Printf("Viewed: marked %d %s on #%d\n", len(files), core.Plural(len(files), "file"), number)
	}
	for _, f := range files {
		fmt.Printf("  %s  %s\n", f.Path, f.Reason)
	}
	return nil
}
