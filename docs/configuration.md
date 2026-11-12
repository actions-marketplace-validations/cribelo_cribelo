# Configuration

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
