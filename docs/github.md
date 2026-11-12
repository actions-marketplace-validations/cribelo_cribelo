# Cribelo on GitHub pull requests

Save this as `.github/workflows/cribelo.yml`:

```yaml
name: cribelo
on: pull_request
permissions:
  contents: read
  pull-requests: write
jobs:
  cribelo:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: cribelo/cribelo@v2
```

The plan is posted as a single comment and updated on each push. Each line range in it links to the diff of the pull request. The job never fails on the content of the change unless you ask for a gate:

```yaml
      - uses: cribelo/cribelo@v2
        with:
          fail-over: 1h
          fail-on: cycle, signature
```

`fail-over: 1h` fails the job when reading the whole change, every file with something to read, would take more than an hour: the change is too large for one reviewer, split it. `fail-on` fails it when the change states a fact of the kinds you name, here an import cycle introduced or an exported signature changed that other files call; any kind listed in [how it is computed](how-it-is-computed.md#facts) works. The gate is set on what the plan measures, never on the score. The job log and the comment say why it failed, one line per breach, such as "reading time 1 h 17, over the 1 h allowed".

A coding agent can be held to the same limit before it opens anything: `cribelo --fail-over 30m` exits with 1 when what it wrote is too long to read. [Cribelo for a coding agent](agents.md) has the hook that does it in Claude Code.

The places to read closely are also marked on the diff, in the Files tab. Each one carries a review comment that says what to check there and why:

> **Read closely.** This change adds 15 branches to `resolveFileDependencies()`, which already had many (complexity 29 → 44). For each new condition, check when it is taken and whether a test takes it. The change touches it again on lines 122-127, 141 and 152.
>
> 83 files depend on this file and it needed 1 fix in the last 90 days.

A function is told once, on the first lines the change touches in it, and what the diff does not show about a file goes with its first comment. Ten places are marked at most, the first ones in reading order, and the comments of the previous run are removed when a new push is reviewed. Both can be changed:

```yaml
      - uses: cribelo/cribelo@v2
        with:
          inline-max: 20   # or inline: false, for the plan alone
```

Outside GitHub Actions, `cribelo --format github-review` prints the body of the review for the [reviews endpoint](https://docs.github.com/en/rest/pulls/reviews#create-a-review-for-a-pull-request), and [`scripts/post-review.sh`](../scripts/post-review.sh) posts it with `gh`.

## Pull requests from forks

GitHub runs a pull request from a fork, or from Dependabot, with a read-only token, so the job cannot comment. Without any setup, the plan still lands in the job summary, and the places to read closely are written as notices of the job, which GitHub shows on the lines of the diff. Notices are plain text, ten at most.

To get the comment and the review comments as on any other pull request, a second workflow posts them with the rights of your repository. Save it as `.github/workflows/cribelo-comment.yml`:

```yaml
on:
  workflow_run:
    workflows: [cribelo]
    types: [completed]
permissions:
  actions: read
  pull-requests: write
jobs:
  comment:
    runs-on: ubuntu-latest
    steps:
      - uses: cribelo/cribelo/comment@v2
```

This workflow checks out nothing and runs no code from the fork. It reads the plan as text, and posts it only if the pull request still points at the commit the plan was built from.
