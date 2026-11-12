---
name: cribelo-review
description: Review a change, a branch or a pull request with Cribelo's reading map. Use when asked to review code, a diff, a branch or a PR ("review this", "relis ce changement", "regarde la PR 412") in a repository where the cribelo command is installed. Cribelo says which lines to read closely, which to glance at, which files are mechanical, and the facts the diff does not show; this skill turns that into a review.
---

# Reviewing with Cribelo

Cribelo does not find bugs. It says where the reading time of a change goes, and states a few facts about it that are true by construction. Spend your attention where it says, and say what you did not read.

## Get the map

For the working tree or the current branch against the base:

```sh
cribelo --format json            # add --base <ref> when the base is not the default branch
```

For a pull request or a GitLab merge request you have not checked out (needs `gh`, or `glab` on GitLab):

```sh
cribelo --format json pr <number>
```

If `cribelo` is not installed, say so and review without it. Do not guess what it would have said.

In the JSON, read:

- `change_minutes`: what the whole change costs a human to read. Say it at the top of the review. Over thirty minutes, say that the change is long to review and that the last files will get less attention than the first.
- `files`, in order: this is the reading order. Each has `read_ranges` (read closely), `glance_ranges` (glance), `reasons`, and `hunks` with a `nature`.
- `facts` (absent when there are none): what the diff does not show. An import cycle the change closes, an exported signature that changed and who calls it, a function that is a clone of one the change did not touch, changed lines no test covers.
- `skipped`: files set aside, with the reason. `moves` and `natures`: what is mechanical (moved, renamed, reformatted).

`cribelo explain <path>` gives every signal behind one file when its place surprises you.

## Read in that order

1. **The facts first.** Each one is a question for the author, and you can check it. A signature with eleven callers: open the callers the change did not touch. A clone: open the other copy and ask why there are two. A cycle: name the import that closes it.
2. **The `read_ranges`, file by file, in the order given.** These are the functions that gained a decision or stay complex. For each new condition, ask when it is taken and whether a test takes it.
3. **The `glance_ranges`.** Glance is not skip: lines in a function that gained no branch are repaired half as often as the rest, and still hold a third of the later fixes. Read them fast, and stop where something looks out of place.
4. **The mechanical part.** Check one or two moves and renames against what Cribelo says they are, then trust the rest. Say that you sampled it.

## What to look for

Most review comments in practice are not about bugs. They are about coherence: a name that does not follow the names around it, a helper that already exists elsewhere in the repository, a module that starts knowing about another it had no business with, a second way of doing what the code already does one way. Look for those before looking for defects. Search the repository before saying something is new.

## Write the review

- Open with the reading time and what you read: "41 min of reading. I read the 7 files of the plan closely, glanced at the rest, sampled 2 of the 9 moves."
- One finding per place, with the file and line, what you see, and what you would change. Coherence findings and defects are both welcome; say which is which.
- Put the facts Cribelo stated in their own short list, each with what you found when you checked it.
- Close with what you did not read, if anything.

Do not present a file's `score` or `level` as a probability of a bug. On every label measured, the size of a change predicts later fixes as well as the score does. The score orders the reading, nothing more.
