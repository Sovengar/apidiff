# context.md — apidiff

Everything the executor needs. Nothing else is required reading.

## Environment

- Location: the repository root (already `git init`, branch `main`, docs only).
- Go: **always** run through mise: `mise exec go -- go …` (system `/usr/bin/go` disagrees with GOROOT).
  Verified: `go version go1.27.3 linux/amd64`.
- No CI yet. Do not create tags.
- Network to `proxy.golang.org` works.

## What to build

One module, one binary:

```
apidiff capture --url URL --out FILE [--force]
apidiff check   --base FILE|URL --rev FILE|URL
                [--format text|json|markdown]
                [--fail-on ERR|WARN|INFO]
                [--ignore rule-id[,rule-id...]]
```

Exit codes: `0` ok, `1` gate fired (`--fail-on` and findings at/above that level),
`2` usage / IO / parse error. Default is report-only: no flag, always `0` on a
successful comparison.

Layout:

```
cmd/apidiff/main.go        thin: os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
internal/contract/         the engine: load, compare, classify, render
internal/contract/testdata v1.yaml, v2.yaml, additive.yaml, malformed.yaml
internal/cli/              flags, exit codes, output
internal/httpload/         fetch a URL into bytes (capture + --rev URL)
Makefile                   build, test, check, install, clean
AGENTS.md                  short: what it is, stack, commands, "run make build"
README.md                  usage
```

Module name: `apidiff`.

## Engine (verified API of oasdiff v1.37.0)

Import only these three packages; they pull the rest as indirect deps:

```go
import (
    "github.com/getkin/kin-openapi/openapi3"
    "github.com/oasdiff/oasdiff/checker"
    "github.com/oasdiff/oasdiff/diff"
    "github.com/oasdiff/oasdiff/load"
)
```

```go
// One fresh loader per document: the loader caches documents by name.
loader := openapi3.NewLoader()
loader.IsExternalRefsAllowed = false
base, err := load.NewSpecInfoFromData(loader, baseBytes, "baseline.yaml")
rev, err  := load.NewSpecInfoFromData(openapi3.NewLoader(), revBytes, "revision.yaml")

report, sources, err := diff.GetWithOperationsSourcesMap(diff.NewConfig(), base, rev)
// report may be non-nil with an empty Diff; checker tolerates nil.
cfg := checker.NewConfig(checker.GetAllChecks())
changes := checker.CheckBackwardCompatibilityUntilLevel(cfg, report, sources, checker.INFO)
localize := checker.NewDefaultLocalizer()
for _, c := range changes {
    id   := c.GetId()            // e.g. api-path-removed-without-deprecation
    path := c.GetPath()          // e.g. /widgets/{id}
    op   := strings.ToLower(c.GetOperation())
    text := ansi.Strip(c.GetText(localize))
    breaking := c.IsBreaking()
}
```

Three real traps, all confirmed by running the library:

1. **`GetText` returns ANSI** (`\x1b[1m'color'\x1b[0m`). Strip with
   `github.com/charmbracelet/x/ansi` before it reaches stdout, JSON or markdown.
   A test must assert no `\x1b` in output.
2. **`checker.CheckBackwardCompatibility` filters to WARN**, so additive (INFO)
   changes disappear. Use `CheckBackwardCompatibilityUntilLevel(..., checker.INFO)`.
3. **Use a fresh loader per spec.** Reusing one can mix cached documents. The
   two specs may share a file name, so never pass the same loader twice.

Levelless detail: `c.IsBreaking()` is true for ERR and WARN; `INFO` changes are
non-breaking. Keep both lists.

Rule ids observed on the fixtures (do not hardcode them in the engine, only in
tests):

| id | level |
|---|---|
| `api-path-removed-without-deprecation` | breaking |
| `request-property-became-required` | breaking |
| `endpoint-added` | non-breaking |
| `new-optional-request-property` | non-breaking |

`--fail-on` mapping: findings with `ERR` count for `ERR`, `WARN` and `INFO`
(ordered ERR > WARN > INFO). `WARN` counts for `WARN` and `INFO`. `INFO` only
for `INFO`. `c.IsBreaking()` does **not** distinguish them, so map by comparing
`c.GetLevel()` with the parsed target level:

```go
target, err := checker.NewLevel(flagValue) // "ERR"|"WARN"|"INFO"
fired := len(filter(changes, func(c checker.Change) bool { return c.GetLevel() >= target })) > 0
```

## Output contracts

`--format text` (default) — human, stable order (breaking first, then
non-breaking; each sorted by path, method, id, pointer):

```
api/openapi.yaml: 2 breaking, 2 non-breaking

breaking
  api-path-removed-without-deprecation  GET /widgets/{id}  (/paths/~1widgets~1{id}/get)
      API path removed without deprecation
  request-property-became-required       POST /widgets  (/paths/~1widgets/post)
      the request property 'color' became required

non-breaking
  endpoint-added  GET /widgets/search  (/paths/~1widgets~1search/get)
```

With zero changes: exactly one line, `no contract changes`.

`--format json` — one JSON document:

```json
{
  "base": "baseline.yaml",
  "revision": "http://localhost:8090/v3/api-docs",
  "breaking": [{"id": "...", "level": "ERR", "path": "/widgets", "method": "get",
                "pointer": "/paths/~1widgets/get", "text": "..."}],
  "nonBreaking": [{"id": "...", "level": "INFO", "path": "...", "method": "...",
                   "pointer": "...", "text": "..."}],
  "summary": {"breaking": 2, "nonBreaking": 2}
}
```

`base`/`revision` echo what the caller passed, not a resolved path.

`--format markdown`:

```markdown
## API contract

`baseline.yaml` → `http://localhost:8090/v3/api-docs`

**2 breaking**, 2 non-breaking

- `api-path-removed-without-deprecation` `/paths/~1widgets~1{id}/get` — API path removed without deprecation
- `request-property-became-required` `/paths/~1widgets/post` — the request property 'color' became required
```

Non-breaking entries appear only as a count.

Pointer rule: `/paths/` + JSON-pointer-escaped path (`~` → `~0`, `/` → `~1`) +
`/` + lowercase method when the method is non-empty.

## capture

- `GET --url`, 15s timeout, follow redirects, reject non-2xx with the status in
  the message.
- Accept `application/json` or `application/yaml`; sniff by content, not header.
- Validate: top-level `openapi` or `swagger` key. If missing → exit 2, message
  `response from <url> is not an OpenAPI document`, **and write no file**.
- Write pretty JSON (`json.Indent` on the raw body when it is already JSON;
  otherwise convert YAML → JSON via `go.yaml.in/yaml/v3` marshal/unmarshal).
- Atomic write: temp file in the target directory + `os.Rename`.
- Refuse to overwrite an existing file unless `--force` → exit 2.

## Fixtures to create (copy from the verified run)

`internal/contract/testdata/v1.yaml` — OpenAPI 3.0.3, paths `/widgets` GET+POST
and `/widgets/{id}` GET, `NewWidget` requires `name`, optional `color`.

`v2.yaml` — removes `/widgets/{id} GET`, makes `color` required, adds
`/widgets/search GET`, adds optional `notes`.

`additive.yaml` — v1 plus `/owners GET` only.

`malformed.yaml` — `openapi: 3.0.3\npaths: [oops\n`.

`v1.yaml` and `v2.yaml` reproduce the original verified run: v2 removes
`GET /widgets/{id}`, makes `color` required on `POST /widgets`, and adds
`GET /widgets/search` plus the optional `notes` property.

## Tests (mandatory, behavior-first)

Write the test first for each scenario in `docs/behavior.feature`, then the code.
Coverage must include:

- engine: breaking/non-breaking split, ids, pointers, deterministic sort, no
  ANSI, identical specs → empty, malformed spec → error value naming the file,
  added/removed spec semantics (document may be empty on one side → report
  `api-removed`/`api-added`).
- CLI: every exit code (0, 1, 2), each `--format`, `--ignore`, missing flags,
  missing file, `--fail-on` at each level.
- capture: `httptest` server serving a spec, HTML response, non-2xx, existing
  file without `--force`, `--force`, and "no file written" on rejection.
- `--rev URL` path.

Run: `mise exec go -- go test -race ./...` and `make check`.

## House rules

- Wrap errors with context; never print a Go stack trace to the user.
- Diagnostics on stderr; only the report on stdout.
- No new dependency beyond: oasdiff v1.37.0, kin-openapi (indirect, but import
  it directly), charmbracelet/x/ansi, go.yaml.in/yaml/v3.
- Do not touch any other repository. Your edits stay inside this one.
- Commit in small atomic commits on `main`.
