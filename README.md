<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/banner-dark.png">
    <img src="docs/banner.png" width="520" alt="Cribelo: a flamingo standing on a GitHub contribution grid, holding one square in its beak">
  </picture>
</p>

<h3 align="center">Too many pull requests. Know what to read.</h3>

<p align="center"><img src="docs/plan.svg" alt="cribelo in a terminal: 34 files changed, 13 to read in about 45 minutes, each file with the line ranges to read closely and the ones to glance at, then bars for complexity, coupling and recent commits, the totals, the facts and 21 skipped files" width="900"></p>

# Cribelo

Agents open pull requests faster than anyone can read them. Cribelo reads the change before you do and answers three questions: what is left to read once the mechanical part is set aside, what the diff does not show, and where to start.

```sh
brew install cribelo/tap/cribelo
cribelo                      # on the branch you want reviewed
```

It runs on your machine, from the git checkout. There is no service, no account and no language model, and the same change gives the same plan everywhere. It reads Go, PHP, Python, Rust, TypeScript, Java and C#.

Without Homebrew: `curl -fsSL https://raw.githubusercontent.com/cribelo/cribelo/main/install.sh | sh`, or a binary for Linux, macOS and Windows from the [releases page](https://github.com/cribelo/cribelo/releases).

## What is left to read

One pull request of ruff, "Split up types/class.rs", changes 10,595 lines. 8,424 of them only moved, and git sees no renamed file. Cribelo pairs each function that left a file with the one that arrived in another, and gives you the lines that changed on the way.

Renames, formatting, comments and imports are set aside the same way. What remains is split in two: **a function that gained no decision and stays simple is glanced at, the rest is read closely.** The rule was set on 130,998 functions touched in 28 open source repositories. Half of the lines changed inside functions fall on the glance side, in functions that were fixed later 2.2 times less often than the others. Less often is not never, so the plan lists those lines too. Glance is not skip.

## What the diff does not show

```
Worth knowing  true by construction, not predicted
  tsdb/chunkenc/histogram_st.go  appendable() is 100% similar to tsdb/chunkenc/histogram.go:appendable(), which this change does not touch
```

The diff shows one copy. The other, which nobody opened, is the one that will miss the fix.

Cribelo states a handful of facts of that kind: an import cycle the change closes, an exported function whose parameters changed and the files that call it, a module that depends on another for the first time, a function already fixed several times this year, changed lines no test runs. Each is read from the import graph, the history or a coverage report. None is a prediction.

## Where to start

The files come grouped by dependency, the one the others use first, each test right after the code it exercises. You know what a function does before you meet its callers.

`cribelo --budget 15m` keeps what fits in the time you have. `cribelo explain path/to/file` says why a file is where it is.

## Which pull request first

```
$ cribelo inbox
cribelo inbox · 3 pull requests wait for your review, about 1 h 31 of reading

  #212   4 min  fix(engine): read a Rust item         ada    today
  #205  10 min  docs: how the reading time is told    linus    2 d
   #98  1 h 17  feat: who uses this library           grace    9 d
```

The shortest come first: ten minutes there unblock someone today. `cribelo pr 98` then prints the plan of one of them without touching your checkout. Both work on GitHub and on GitLab, through `gh` or `glab`.

## When an agent wrote it

An agent writes a thousand lines as easily as fifty. You still read at the same speed.

```
$ cribelo --fail-over 30m
…
Gate: failed
  reading time 44 min, over the 30 min allowed
```

The command exits with 1. A ten-line hook runs it in Claude Code before the agent opens a pull request: over the limit, the agent has to cut its work into pieces you have time to read. [Cribelo for a coding agent](docs/agents.md) has the hook, an MCP server that gives the agent the plan, and a skill that makes it review in the order of the map.

## In a pull request

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

The plan is posted as one comment, updated on each push, and the places to read closely are marked on the diff. The job never fails unless you ask: [a time limit, facts that fail the job, pull requests from forks](docs/github.md).

## In a GitLab merge request

```yaml
include:
  - remote: https://raw.githubusercontent.com/cribelo/cribelo/v2/ci/gitlab-ci.yml
```

One line in `.gitlab-ci.yml`, on gitlab.com or on your own instance. [The note on the merge request, the options, runners without access to github.com](docs/gitlab.md).

## What it does not do

Cribelo does not find your bugs. It tells you where your reading time goes, and it shows its work: [how it is computed](docs/how-it-is-computed.md) gives every rule and every weight, so that you can contest them, and [`cribelo backtest`](docs/backtest.md) replays the history of your repository to say which of the files fixed later the plan would have had you read.

[Every command](docs/commands.md) · [Configuration](docs/configuration.md) · [Working on Cribelo](docs/development.md)

MIT license. The analysis is the `cribelo` package of [AST Metrics](https://github.com/ast-metrics/ast-metrics).
