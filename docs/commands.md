# The commands

```sh
cribelo                      # plan for the working tree against the default branch
cribelo --base origin/develop
cribelo --budget 15m         # as many files as fit in fifteen minutes
cribelo --top 5              # exactly five files
cribelo explain path/to/file # every signal, hunk and fact behind one file
cribelo skip                 # the skipped files, with the reason
cribelo describe             # the structural summary, in Markdown, for the pull request description
cribelo inbox                # the pull requests waiting for you, the shortest first
cribelo pr 412               # the plan of a pull request, without checking it out
```

`cribelo describe` prints what the author of the change rarely writes down and the reviewer always wants: the signatures that changed and who calls them, what moved where, the share of mechanical lines, and the suggested reading order. Paste it into the description.

## Which pull request first

```
$ cribelo inbox
cribelo inbox · 3 pull requests wait for your review, about 1 h 31 of reading

  #212   4 min  fix(engine): read a Rust item         ada    today
  #205  10 min  docs: how the reading time is told    linus    2 d
   #98  1 h 17  feat: who uses this library           grace    9 d
```

The shortest come first: ten minutes there unblock someone today. The list comes from `gh`, or from `glab` when the remote is on GitLab, so the command line of your forge must be installed and logged in; `--all` takes every open pull request rather than those that ask for you. A head already measured is not measured again. On a self-hosted GitLab under the name of your company, `glab` alone on the machine is enough to tell; when `gh` is there too, say it once with `git config cribelo.forge gitlab`.

`cribelo pr 98` prints the plan of one of them without touching your checkout. With `--viewed`, the files the plan sets aside as mechanical are ticked "Viewed" in the Files tab of the pull request, and what is left unticked is what there is to read; `--dry-run` lists them first. GitLab has `cribelo pr` too, without `--viewed`.

## Why a file is where it is

`cribelo explain` prints every signal behind one file, with its weight in the decision:

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

The transforms, the default weights and the clone detection parameters are in [how it is computed](how-it-is-computed.md).
