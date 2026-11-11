<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/banner-dark.png">
    <img src="docs/banner.png" width="520" alt="Cribelo: a flamingo standing on a GitHub contribution grid, holding one square in its beak">
  </picture>
</p>

<h3 align="center">What to read, what to skip.</h3>

<p align="center"><img src="docs/plan.svg" alt="cribelo in a terminal: 34 files changed, 13 to read in about 45 minutes, each file with the line ranges to read closely and the ones to glance at, then bars for complexity, coupling and recent commits, the totals, the facts and 21 skipped files" width="900"></p>

# Cribelo

Cribelo draws the reading map of a pull request: how many lines are left to read once moves, renames, formatting and imports are set aside, what the diff does not show about the structure of the code, and in which order to read the files. It says why at every step. The name is the Latin *cribellum*, a small sieve.

```
$ cribelo --base origin/main
cribelo · 34 files changed vs origin/main (merge-base b2f83dc)

Read 13 of 34 changed files, about 45 min.

                                                           lines to read closely               lines to glance at
   1  internal/analyzer/library_reach.go             new   30-35, 66-265                       16-26, 39-57
   2  internal/analyzer/library_conformance_test.go  test  98-140                              25-93
   3  internal/analyzer/library_reach_test.go        test  10-63, 85-144                       67-80
   4  internal/engine/util.go                              85-112
   5  internal/analyzer/file_dependency_analyzer.go        97-101, 122-127, 141, 152, +4 more  22-27
   6  internal/engine/php/deps/resolver.go           new   25-43, 57-69                        15-22, 46-51, 75-76
  …
  13  internal/analyzer/community_graph.go                 112, 217, 233, 250, +1 more         line 86

Why these files
                                   complexity  used by    commits 90d        clone  test changed
   1  library_reach.go             ▮▮▮▯▯ 17    ▮▮▮▮▮ 83   ▮▮▯▯▯ 3                   ✓
   4  util.go                      ▮▮▯▯▯ 10    ▮▮▮▮▮ 111  ▮▮▮▯▯ 6, 3 fixes          ✗
   5  file_dependency_analyzer.go  ▮▮▮▮▮ 44    ▮▮▮▮▮ 83   ▮▮▯▯▯ 4, 1 fix            ✓
   6  php/deps/resolver.go         ▮▮▯▯▯ 9     ▮▯▯▯▯ 1    ▮▯▯▯▯ 1            84%    ✓
   8  java/deps/resolver.go        ▮▮▮▯▯ 11    ▮▯▯▯▯ 1    ▮▮▯▯▯ 3, 1 fix     100%   ✓
  10  who_uses_command.go          ▮▮▮▯▯ 18    ▮▮▯▯▯ 3    ▮▮▯▯▯ 3, 1 fix            ✓
  12  community_aggregator.go      ▮▯▯▯▯ 5     ▮▮▮▮▮ 83   ▮▮▮▮▯ 11, 2 fixes         ✓
  13  community_graph.go           ▮▮▮▮▮ 51    ▮▮▮▮▮ 83   ▮▯▯▯▯ 2                   ✓

Totals  688 lines to read closely, 280 to glance at (20 and 60 lines a minute)
Set aside in the whole change: 11 import hunks.

Worth knowing  true by construction, not predicted
  internal/analyzer/aggregator.go  internal/analyzer depends on internal/engine/php/deps for the first time
  internal/engine/typescript/deps/resolver.go  internal/engine/typescript/deps depends on internal/engine/typescript/module for the first time
  internal/analyzer/library_reach.go  15 of 161 changed lines not run by any test, according to pr196-coverage.out
  internal/command/who_uses_command.go  49 of 162 changed lines not run by any test, according to pr196-coverage.out
  internal/engine/util.go  25 of 25 changed lines not run by any test, according to pr196-coverage.out

Skip (21)
  10 ranked lower · 7 tests only · 4 no measurable change    cribelo skip

Weights: generic, not yet calibrated on this repository (run: cribelo calibrate)
```

Three things happen here.

Every hunk of the diff gets a nature. A function that left one file and arrived in another is a move; a block cut out of a function that shrank is an extraction; a consistent `oldName → newName` across the change is a rename; the rest is formatting, comments, imports, or a change of behavior. The code lines of behavior are then split: a line in a function that gained no decision and stays simple, or outside every function, is glanced at; the rest is read closely. The plan gives the lines of each kind file by file, as ranges, and a file whose every hunk is mechanical is set aside with its reason.

The files to read are grouped by dependency, the file the others use first, each test right after the file it exercises. A reader who knows what a function does before meeting its callers finds more, and gets tired later.

When there is something to say, a "Worth knowing" block states what the diff cannot show and a reader can check in a minute:

```
Worth knowing  true by construction, not predicted
  tsdb/chunkenc/histogram_st.go  appendable() is 100% similar to tsdb/chunkenc/histogram.go:appendable(), which this change does not touch
```

A dependency cycle the change introduces, a module depending on another for the first time, a library nobody used before, an exported function whose parameters changed and the files calling it, a new export nothing calls yet, a touched function that is a clone of one the change leaves alone, a function fixed several times this year, changed lines that no test runs when a coverage report is around. None of these is a prediction. Each is true by construction of the graphs and the history it comes from, and Cribelo shows at most a handful.

Within a group, the order falls back on a score: the cyclomatic complexity added by the touched functions, the most complex of them, how many files import the file, whether a touched function is a clone, whether a test changed with it, how often the file was committed in the last ninety days and how many of those commits were fixes. "Why these files" shows them file by file, a bar and its value. The reading list holds every file at the high level, and at least three. `--budget 15m` caps it to what fits in the time you have, `--top 5` forces a count.

Everything runs locally from the git checkout. There is no service, no account and no language model, and the same change gives the same plan on every machine.

## Install

```sh
brew install cribelo/tap/cribelo
```

Without Homebrew: `curl -fsSL https://raw.githubusercontent.com/cribelo/cribelo/main/install.sh | sh`. Or `go install github.com/cribelo/cribelo@latest`, which needs a C compiler for the tree-sitter parsers. Binaries for Linux, macOS and Windows are on the [releases page](https://github.com/cribelo/cribelo/releases). Supported languages: Go, PHP, Python, Rust, TypeScript, Java and C#.

```sh
cribelo                      # plan for the working tree against the default branch
cribelo --base origin/develop
cribelo --budget 15m         # as many files as fit in fifteen minutes
cribelo --top 5              # exactly five files
cribelo explain path/to/file # every signal, hunk and fact behind one file
cribelo skip                 # the skipped files, with the reason
cribelo describe             # the structural summary, in Markdown, for the pull request description
```

`cribelo describe` prints what the author of the change rarely writes down and the reviewer always wants: the signatures that changed and who calls them, what moved where, the share of mechanical lines, and the suggested reading order. Paste it into the description.

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

The plan is posted as a single comment and updated on each push. Each line range in it links to the diff of the pull request. The job never fails on the content of the change unless you ask for a gate:

```yaml
      - uses: cribelo/cribelo@v2
        with:
          fail-over: 1h
          fail-on: cycle, signature
```

`fail-over: 1h` fails the job when reading the whole change, every file with something to read, would take more than an hour: the change is too large for one reviewer, split it. `fail-on` fails it when the change states a fact of the kinds you name, here an import cycle introduced or an exported signature changed that other files call; any kind listed under the facts below works. The gate is set on what the plan measures, never on the score. The job log and the comment say why it failed, one line per breach, such as "reading time 1 h 17, over the 1 h allowed".

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

Outside GitHub Actions, `cribelo --format github-review` prints the body of the review for the [reviews endpoint](https://docs.github.com/en/rest/pulls/reviews#create-a-review-for-a-pull-request), and `scripts/post-review.sh` posts it with `gh`.

### Pull requests from forks

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

## In a GitLab merge request

GitLab keeps every job in a single `.gitlab-ci.yml`, so there is nothing to install. Include the job:

```yaml
include:
  - remote: https://raw.githubusercontent.com/cribelo/cribelo/v2/ci/gitlab-ci.yml
```

It runs on merge request pipelines, in the `test` stage. The plan goes to the job log and to an artifact linked from the merge request as "cribelo plan". To get it as a note, updated on each push, store a token with the `api` scope as a masked `CRIBELO_TOKEN` variable; the job token of GitLab can read notes but not write them. A project access token with the Reporter role is the cleanest choice, but on GitLab.com it needs a Premium or Ultimate plan: on the Free plan, use a personal access token of an account that is a member of the project. Options are variables, named after the inputs of the Action:

```yaml
cribelo:
  variables:
    CRIBELO_BUDGET: 30m
    CRIBELO_FAIL_ON: cycle, signature
```

A merge request from a fork runs in the fork, without your variables: the plan stays in the artifact. If your pipeline has no `test` stage, set `stage:` on the `cribelo` job the same way. If your jobs run in branch pipelines, each push to a branch with an open merge request now starts two pipelines, the usual one and a merge request pipeline holding only cribelo. GitLab's [`workflow:rules` examples](https://docs.gitlab.com/ci/yaml/workflow/#switch-between-branch-pipelines-and-merge-request-pipelines) move everything into merge request pipelines.

## The facts

| Fact | How it is established |
|---|---|
| introduces a cycle | the import graph of the repository after the change holds a path back from a new dependency to the file |
| depends on a module for the first time | no file of the directory imported a file of the other directory before the change |
| first use of a library | no file of the repository imported that module before the change |
| changed its parameters, called from N files | the parameter list of an exported function differs between the two versions; callers are counted by name |
| new exported functions called nowhere yet | exported functions the change adds whose name no other file calls |
| similar to a function the change does not touch | the clone index, when the twin is not in the change |
| fixed N times in the past year | `git log -L` on the function's lines in the base version, fixes recognized as in the backtest |
| changed lines not run by any test | a coverage report found at the root (lcov, Cobertura, Clover or a Go profile), or given with `--coverage` |

Exported means uppercase in Go, no leading underscore in Python, `public` or absent visibility in PHP, `export` in TypeScript, `pub` in Rust, `public` in Java and C#. A caller is counted when a file holds the function's name followed by a parenthesis, so the count is an upper bound in the languages where a name is not a symbol.

## The signals

| Signal | Where it comes from |
|---|---|
| lines added, lines deleted | the diff |
| complexity added | cyclomatic complexity of the touched functions, after minus before |
| max complexity | the most complex function touched |
| afferent coupling | the import graph of the repository, files outside the directory only |
| fixes, churn | commits on the file in the ninety days before the change, and how many of them were fixes |
| new file, untested | the diff and the tree |
| clone | similarity of a touched function with any other function of the project, detected on normalized token fingerprints |

Each signal has a weight, the weighted sum goes through a logistic function, and the files are sorted by the result. A score of 0.70 or more is high, 0.40 or more medium; the high files are the reading list. The reasons printed in the plan are the three signals that contributed most. `cribelo explain` prints all of them:

```
$ cribelo explain internal/cribelo/git.go
internal/cribelo/git.go  score 0.80 (high), ~21 min, not in the reading list

Signals  value → weight in the decision (positive pushes up)
  lines added                                                   404  +2.10
  decision points added (new or grown functions)                 55  +2.01
  highest cyclomatic complexity among touched functions          10  +0.72
  file is new                                                     1  +0.30
  files outside its directory depending on this file              1  +0.28
  ...
```

The transforms, the default weights and the clone detection parameters are in [how it is computed](docs/how-it-is-computed.md).

## Measuring it on your own history

`cribelo backtest` replays the first-parent history of the base branch. For each past change it rebuilds the plan from the code of that time, then looks at the ninety days that followed: a file counts as fixed when a commit whose message says fix, revert, regression or the equivalent in a few languages touches it. The output is the share of those files the reading list contained, and the share of the changed lines you would have read to get there, for the top three files, for the rule the plan applies (every high file, at least three), and for reading the largest files first.

```
$ cribelo backtest
79 changes qualified, 2023-10-22 → 2026-07-28: 1007 code files, 151 of them (15%) later touched by a fix.

generic weights, all qualified changes
  Reading the top-3 files covered 40% of the files later fixed, for 59% of the changed lines.
  Reading the 3 largest files instead: 36% of the files later fixed, for 63% of the lines.
  Reading every high file, at least 3 (3.7 files a change): 44% of the files later fixed, for 71% of the lines.
```

These figures are from the AST Metrics repository. A fix commit touching a file later is a noisy label, the fix may be elsewhere, so read the result as a comparison with the naive plan rather than as a verdict.

`cribelo calibrate` fits a logistic regression on the oldest 70 % of those changes, scores it on the most recent 30 %, and writes the weights to `.cribelo/weights.json` only when they beat the defaults on that held-out part, with the top three files and with every high file. The high and medium thresholds move with the weights, so that a calibrated model flags the same share of files as the generic one. Commit the file; every plan then uses it. This is a quarterly job, not a CI step.

## Configuration

`.cribelo.yaml` at the root of the repository, all keys optional:

```yaml
base: origin/develop
top: 3                      # the least number of files to read
fail_on: [cycle, signature]  # facts that fail the gate; none by default
fail_over: 1h               # reading time of the whole change that fails the gate
coverage: coverage/lcov.info  # report to read the untested changed lines from; none to skip
ignore:
  - "migrations/**"
  - "*.generated.go"
fix_pattern: "JIRA-\\d+"    # commits matching this also count as fixes
```

Fix commits are recognized by `fix:` and `revert:` conventional types, the 🐛 🚑 🩹 ⏪ gitmoji, and the usual words in English, French, Spanish, Portuguese and German. `fix_pattern` adds a convention of your own, a tracker key for instance.

## Source

The analysis is the `cribelo` package of [AST Metrics](https://github.com/ast-metrics/ast-metrics): the parsers, the complexity model, the dependency graph, the facts and the reading order come from there, and this repository pins a revision of it in `go.mod`. What you read is composed here: the command line in `internal/cli`, the terminal plan, the pull request comment and the description in `internal/render`. A change of wording or layout is a pull request on this repository; a change of what is measured is one on AST Metrics. This repository also holds the Action, the installer, the documentation and the issues. MIT license.

To work on the output against a local checkout of the engine, point a `go.work` at it (the file is ignored by git):

```sh
go work init . ../ast-metrics
go test ./... && go run . -C ../some-repository
```
