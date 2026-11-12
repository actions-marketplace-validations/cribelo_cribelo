# Cribelo for a coding agent

An agent writes a thousand lines as easily as fifty, and the person who reviews them reads at the same speed as before. Cribelo gives the agent the number it lacks: how long the change it just wrote takes to read.

## The limit

```sh
cribelo --fail-over 30m
```

exits with 1 when the change takes more than thirty minutes to read, and says so in one line: "reading time 1 h 17, over the 30 min allowed".

To hold an agent to it in Claude Code, block the opening of a pull request until the change passes. In `.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "bash \"${CLAUDE_PROJECT_DIR}/.claude/hooks/cribelo-gate.sh\"" }
        ]
      }
    ]
  }
}
```

and in `.claude/hooks/cribelo-gate.sh`:

```bash
cmd=$(jq -r '.tool_input.command // ""')
case "$cmd" in
  *"gh pr create"*|*"glab mr create"*|*"merge_request.create"*) ;;
  *) exit 0 ;;
esac
if out=$(cribelo --fail-over 30m 2>&1); then
  exit 0
fi
printf '%s\n' "$out" >&2
exit 2
```

The three patterns are `gh pr create` on GitHub, and on GitLab `glab mr create` or a `git push -o merge_request.create`. Exit code 2 stops the command, and what the script wrote on its error output goes back to the agent, which knows by how much the change is over and can cut it into smaller pull requests before trying again. Each of them passes through the same limit.

## As a tool the agent calls

`cribelo mcp` serves the plan over the Model Context Protocol, on standard input and output, as two tools: `reading_plan`, the reading time of the change with what to read closely, and `explain`, the signals behind one file. Nothing it does writes to the repository. In `.mcp.json` at the root of the project:

```json
{
  "mcpServers": {
    "cribelo": { "command": "cribelo", "args": ["mcp"] }
  }
}
```

The hook is the one to start with. A tool is called when the agent thinks of it; a hook runs every time.

## When the agent is the reviewer

`skills/cribelo-review` is a skill for Claude Code. Asked to review a change or a pull request, the agent runs Cribelo first, checks the facts it states, reads the ranges to read closely in the order given, glances at the rest, and says in its review what it read and what it sampled. Copy the directory into `.claude/skills/` of the project, or into your own skills directory.
