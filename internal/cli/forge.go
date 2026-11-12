package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// The inbox and `cribelo pr` read the open changes of a forge through its
// own command line, which already holds the login of the user: gh on
// GitHub, glab on GitLab. Everything after that (the fetch of the heads,
// the plan, the cache) is the same on both.

const (
	github = "github"
	gitlab = "gitlab"
)

// forge is where the pull requests of a repository live.
type forge struct {
	Kind   string
	Remote string
	// Host and Project locate the repository on the forge: gitlab.com and
	// group/subgroup/project. Only GitLab needs them, gh reads the remote
	// by itself.
	Host    string
	Project string
}

// parseRemote splits the address of a remote into its host and the path of
// the repository, without the .git suffix. Both are empty when the address
// is not one of a server (a local path).
func parseRemote(address string) (host, path string) {
	address = strings.TrimSpace(address)
	if strings.Contains(address, "://") {
		u, err := url.Parse(address)
		if err != nil {
			return "", ""
		}
		host, path = u.Hostname(), u.Path
	} else if at := strings.Index(address, ":"); at > 0 && !strings.Contains(address[:at], "/") {
		// The scp form: git@gitlab.com:group/project.git
		host, path = address[:at], address[at+1:]
		if user := strings.LastIndex(host, "@"); user >= 0 {
			host = host[user+1:]
		}
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	if host == "" || path == "" {
		return "", ""
	}
	return host, path
}

// forgeOf tells the forge of a host by its name; empty when the name says
// nothing (a self-hosted instance).
func forgeOf(host string) string {
	switch host = strings.ToLower(host); {
	case strings.Contains(host, "github"):
		return github
	case strings.Contains(host, "gitlab"):
		return gitlab
	}
	return ""
}

// loggedIn tells whether the command line of a forge is installed and, for
// gh, logged in to the host. A variable, for the tests.
var loggedIn = func(tool, host string) bool {
	if _, err := exec.LookPath(tool); err != nil {
		return false
	}
	if tool == "gh" {
		return exec.Command("gh", "auth", "status", "--hostname", host).Run() == nil
	}
	return true
}

// selfHosted tells the forge of a host whose name says nothing, a GitLab
// or a GitHub Enterprise under the name of the company. In order: the
// host glab itself is pointed at by GITLAB_HOST, a host gh is logged in
// to, then glab when it is the one installed.
func selfHosted(host string) string {
	for _, name := range []string{"GITLAB_HOST", "GL_HOST"} {
		if value := os.Getenv(name); value != "" {
			if u, err := url.Parse(value); err == nil && u.Hostname() != "" {
				value = u.Hostname()
			}
			if strings.EqualFold(value, host) {
				return gitlab
			}
		}
	}
	if loggedIn("gh", host) {
		return github
	}
	if loggedIn("glab", host) {
		return gitlab
	}
	return ""
}

// detectForge picks the remote the pull requests live on, origin first,
// and tells its forge. The --forge option wins, then the cribelo.forge
// setting of the repository, then the name of the host, then what the
// command lines installed say of a self-hosted one.
func detectForge(root, forced string) (forge, error) {
	if forced == "" {
		if out, err := gitIn(root, "config", "--get", "cribelo.forge"); err == nil {
			forced = strings.TrimSpace(out)
		}
	}
	if forced != "" && forced != github && forced != gitlab {
		return forge{}, fmt.Errorf("unknown forge %q (github or gitlab)", forced)
	}
	out, err := gitIn(root, "remote", "-v")
	if err != nil {
		return forge{}, err
	}
	var origin, first forge
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		host, project := parseRemote(fields[1])
		f := forge{Kind: forgeOf(host), Remote: fields[0], Host: host, Project: project}
		if forced != "" && host != "" {
			f.Kind = forced
		}
		if fields[0] == "origin" && origin.Remote == "" {
			origin = f
		}
		if first.Remote == "" && f.Kind != "" {
			first = f
		}
	}
	switch {
	case origin.Kind != "":
		return origin, nil
	case first.Kind != "":
		return first, nil
	case origin.Host != "":
		if origin.Kind = selfHosted(origin.Host); origin.Kind == "" {
			return forge{}, fmt.Errorf("cannot tell whether %s is a GitHub or a GitLab: run git config cribelo.forge gitlab (or github)", origin.Host)
		}
		return origin, nil
	}
	return forge{Kind: github, Remote: "origin"}, nil
}

// noun is how the forge calls a change proposed for review.
func (f forge) noun() string {
	if f.Kind == gitlab {
		return "merge request"
	}
	return "pull request"
}

// sign is what the forge writes before the number of one.
func (f forge) sign() string {
	if f.Kind == gitlab {
		return "!"
	}
	return "#"
}

// headRef is the ref the forge keeps the head of a pull request under,
// whether it comes from a fork or not.
func (f forge) headRef(number int) string {
	if f.Kind == gitlab {
		return fmt.Sprintf("refs/merge-requests/%d/head", number)
	}
	return fmt.Sprintf("refs/pull/%d/head", number)
}

// tool checks that the command line of the forge is installed.
func (f forge) tool(command string) error {
	name, where := "gh", "the GitHub command line: install gh (https://cli.github.com) and run gh auth login"
	if f.Kind == gitlab {
		name, where = "glab", "the GitLab command line: install glab (https://gitlab.com/gitlab-org/cli) and run glab auth login"
	}
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s reads the %ss with %s", command, f.noun(), where)
	}
	return nil
}

const pullFields = "number,title,url,headRefOid,baseRefName,isDraft,updatedAt,author"

// list reads the open pull requests: those that ask for the review of the
// user, or all of them.
func (f forge) list(root string, limit int, all bool) ([]pullRequest, error) {
	if err := f.tool("inbox"); err != nil {
		return nil, err
	}
	if f.Kind == gitlab {
		fields := []string{"state=opened", "per_page=" + strconv.Itoa(min(max(limit, 1), 100))}
		if !all {
			out, err := f.glab(root, "user")
			if err != nil {
				return nil, err
			}
			var user struct {
				Username string `json:"username"`
			}
			if err := json.Unmarshal(out, &user); err != nil || user.Username == "" {
				return nil, errors.New("glab api user: no username in the answer, run glab auth login")
			}
			fields = append(fields, "reviewer_username="+user.Username)
		}
		out, err := f.glab(root, f.endpoint("merge_requests"), fields...)
		if err != nil {
			return nil, err
		}
		var merges []mergeRequest
		if err := json.Unmarshal(out, &merges); err != nil {
			return nil, fmt.Errorf("glab api merge_requests: %w", err)
		}
		pulls := make([]pullRequest, 0, len(merges))
		for _, m := range merges {
			pulls = append(pulls, m.pull())
		}
		return pulls, nil
	}
	args := []string{"pr", "list", "--state", "open", "--limit", strconv.Itoa(limit), "--json", pullFields}
	if !all {
		args = append(args, "--search", "review-requested:@me")
	}
	out, err := gh(root, args...)
	if err != nil {
		return nil, err
	}
	var pulls []pullRequest
	if err := json.Unmarshal(out, &pulls); err != nil {
		return nil, fmt.Errorf("gh pr list: %w", err)
	}
	return pulls, nil
}

// view reads one pull request. The id is the one the GraphQL API of GitHub
// knows it by, empty on GitLab.
func (f forge) view(root string, number int) (pull pullRequest, id string, err error) {
	if err := f.tool("pr"); err != nil {
		return pull, "", err
	}
	if f.Kind == gitlab {
		out, err := f.glab(root, f.endpoint("merge_requests/"+strconv.Itoa(number)))
		if err != nil {
			return pull, "", err
		}
		var merge mergeRequest
		if err := json.Unmarshal(out, &merge); err != nil {
			return pull, "", fmt.Errorf("glab api merge_requests: %w", err)
		}
		return merge.pull(), "", nil
	}
	out, err := gh(root, "pr", "view", strconv.Itoa(number), "--json", "id,"+pullFields)
	if err != nil {
		return pull, "", err
	}
	var answer struct {
		pullRequest
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &answer); err != nil {
		return pull, "", fmt.Errorf("gh pr view: %w", err)
	}
	return answer.pullRequest, answer.ID, nil
}

// mergeRequest is what the REST API of GitLab says of a merge request.
type mergeRequest struct {
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	WebURL       string `json:"web_url"`
	Sha          string `json:"sha"`
	TargetBranch string `json:"target_branch"`
	Draft        bool   `json:"draft"`
	UpdatedAt    string `json:"updated_at"`
	Author       struct {
		Username string `json:"username"`
	} `json:"author"`
}

func (m mergeRequest) pull() pullRequest {
	p := pullRequest{Number: m.IID, Title: m.Title, URL: m.WebURL, HeadRefOid: m.Sha, BaseRefName: m.TargetBranch, IsDraft: m.Draft, UpdatedAt: m.UpdatedAt}
	p.Author.Login = m.Author.Username
	return p
}

// endpoint is the path of a resource of the project in the REST API.
func (f forge) endpoint(resource string) string {
	return "projects/" + url.PathEscape(f.Project) + "/" + resource
}

// glab calls the REST API of GitLab through glab, which holds the token.
// The fields go to the query string.
func (f forge) glab(root, endpoint string, fields ...string) ([]byte, error) {
	args := []string{"api", "--hostname", f.Host, "--method", "GET", endpoint}
	for _, field := range fields {
		args = append(args, "--raw-field", field)
	}
	cmd := exec.Command("glab", args...)
	cmd.Dir = root
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		reason := strings.TrimSpace(stderr.String())
		if reason == "" {
			reason = strings.TrimSpace(string(out))
		}
		if reason == "" {
			reason = err.Error()
		}
		return nil, fmt.Errorf("glab api %s: %s", endpoint, reason)
	}
	return out, nil
}
