# Cribelo on GitLab merge requests

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

On a self-hosted GitLab the job talks to your instance, not to gitlab.com. If its runners do not reach github.com, copy [`ci/gitlab-ci.yml`](../ci/gitlab-ci.yml) into a project of yours, include it with `include: project`, and point `CRIBELO_DOWNLOAD_URL` at a copy of the binary.

A merge request from a fork runs in the fork, without your variables: the plan stays in the artifact. If your pipeline has no `test` stage, set `stage:` on the `cribelo` job the same way. If your jobs run in branch pipelines, each push to a branch with an open merge request now starts two pipelines, the usual one and a merge request pipeline holding only cribelo. GitLab's [`workflow:rules` examples](https://docs.gitlab.com/ci/yaml/workflow/#switch-between-branch-pipelines-and-merge-request-pipelines) move everything into merge request pipelines.
