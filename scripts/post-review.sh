#!/usr/bin/env bash
# Posts the lines to read closely as review comments on a pull request, after
# removing those this token posted on an earlier run, so that only the current
# head is marked. Usage: post-review.sh OWNER/REPO PR REVIEW_JSON
# REVIEW_JSON is the output of cribelo --format github-review. Needs GH_TOKEN
# with pull-requests write. Exits non-zero when it cannot post.
set -euo pipefail

repo=$1 pr=$2 review=$3
# The same string ends every comment cribelo writes (internal/render/review.go).
marker='<!-- cribelo-review -->'
body="${RUNNER_TEMP:-/tmp}/cribelo-review.json"

# Only remove comments written by this token: anyone can end a comment with
# the marker. GITHUB_TOKEN cannot read /user, its login is fixed.
if ! login=$(gh api user --jq .login 2>/dev/null) || ! [[ "$login" =~ ^[A-Za-z0-9-]+(\[bot\])?$ ]]; then
  login='github-actions[bot]'
fi
gh api "repos/$repo/pulls/$pr/comments" --paginate \
  --jq ".[] | select(.user.login == \"$login\" and (.body | contains(\"$marker\"))) | .id" |
while read -r id; do
  gh api -X DELETE "repos/$repo/pulls/comments/$id" > /dev/null
done

if [ "$(jq '.comments | length' "$review")" -eq 0 ]; then
  echo "no lines to read closely, no review posted"
  exit 0
fi

# The lines are numbered on the commit the job checked out, the merge commit
# of the pull request on a pull_request event; the review is attached to its
# head, where the diff shows the same lines unless the base also touched the
# file. A review holds at most a few dozen comments without drowning the
# diff: the plan already keeps four spans a file.
head=$(gh api "repos/$repo/pulls/$pr" --jq .head.sha)
jq --arg head "$head" '.commit_id = $head' "$review" > "$body"
gh api "repos/$repo/pulls/$pr/reviews" --input "$body" > /dev/null
