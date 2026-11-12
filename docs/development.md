# Working on Cribelo

The analysis is the `cribelo` package of [AST Metrics](https://github.com/ast-metrics/ast-metrics): the parsers, the complexity model, the dependency graph, the facts and the reading order come from there, and this repository pins a revision of it in `go.mod`. What you read is composed here: the command line in `internal/cli`, the terminal plan, the pull request comment and the description in `internal/render`. A change of wording or layout is a pull request on this repository; a change of what is measured is one on AST Metrics. This repository also holds the Action, the installer, the documentation and the issues. MIT license.

To work on the output against a local checkout of the engine, point a `go.work` at it (the file is ignored by git):

```sh
go work init . ../ast-metrics
go test ./... && go run . -C ../some-repository
```

`go install github.com/cribelo/cribelo@latest` builds it from source; it needs a C compiler for the tree-sitter parsers.
