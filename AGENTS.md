# AGENTS.md — apidiff

Guide for agents with no prior context about this project.

## What it is

A static CLI that compares two OpenAPI documents and reports breaking changes.
It is report-only by default: it gates (exit 1) only when `--fail-on` asks it to.

## Stack

- Go 1.27+, module `apidiff`
- `github.com/oasdiff/oasdiff` v1.37.0 (`diff`, `checker`, `load`) — the engine
- `github.com/getkin/kin-openapi` — loader
- `github.com/charmbracelet/x/ansi` — strips ANSI from checker text
- `go.yaml.in/yaml/v3` — YAML parsing for capture
- Binary name: **`apidiff`**

## Commands

Always run Go through mise: the system Go disagrees with GOROOT.

```bash
make build          # compiles to bin/apidiff (local repo artifact)
make install        # installs to $(PREFIX)/bin/apidiff (default ~/.local/bin)
make test           # go vet + go test -race
make check          # build + vet + test + full-tree compile (local gate)
make clean          # removes bin/
```

Raw equivalents:

```bash
mise exec go -- go test -race ./...
mise exec go -- go build -o bin/apidiff ./cmd/apidiff
```

## Architecture

```
cmd/apidiff/main.go         entry: os.Exit(cli.Run(...))
internal/cli/               flags, exit codes, report and diagnostics split
internal/contract/          load, diff, classify, render (text/json/markdown)
internal/contract/testdata  v1.yaml, v2.yaml, additive.yaml, malformed.yaml
internal/httpload/          fetch a URL into bytes (capture and --rev URL)
```

## Contract

- Exit codes: `0` ok, `1` gate fired by `--fail-on`, `2` usage/IO/parse error.
- Diagnostics on stderr; only the report on stdout; never a Go stack trace.
- `internal/contract` owns oasdiff. Tests hardcode rule ids; production code
  never does.
- Fresh kin-openapi loader per document: the loader caches by name.
- `checker.CheckBackwardCompatibilityUntilLevel(..., checker.INFO)` — the
  plain `CheckBackwardCompatibility` drops INFO additions.
- Checker text contains ANSI; strip it before any output.

## Conventions

- Files: `snake_case.go`; types: `PascalCase`; comments: English.
- `internal/` only — no exported packages.
- Wrap errors with the caller-provided name (file path or URL).
- Tests are behavior-first, written from `docs/behavior.feature`.
- Atomic writes: temp file in the target directory + `os.Rename`.
- Small atomic commits on `main`. Conventional Commits.
