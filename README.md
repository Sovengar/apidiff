# apidiff

Detect breaking changes between two OpenAPI contracts. One static binary, two
commands, report-only by default.

## Install

```bash
make build            # bin/apidiff
make install          # ~/.local/bin/apidiff
```

## Usage

```bash
apidiff check --base FILE|URL --rev FILE|URL [flags]
apidiff capture --url URL --out FILE [--force]
```

### check

Compare a baseline contract with a revision.

| flag | meaning |
|---|---|
| `--base FILE\|URL` | baseline document (required) |
| `--rev FILE\|URL` | revision document (required) |
| `--format text\|json\|markdown` | output format, default `text` |
| `--fail-on ERR\|WARN\|INFO` | gate: exit 1 when a finding is at or above this level |
| `--ignore id[,id...]` | rule ids to omit |

```bash
apidiff check --base baseline.json --rev http://localhost:8080/v3/api-docs
apidiff check --base v1.yaml --rev v2.yaml --fail-on ERR
apidiff check --base v1.yaml --rev v2.yaml --format markdown --ignore endpoint-added
```

Exit codes: `0` ok, `1` gate fired, `2` usage/IO/parse error. Without
`--fail-on` a successful comparison always exits `0`.

### capture

Fetch a live contract and write it as pretty-printed JSON. The response must
have a top-level `openapi` or `swagger` key; anything else is rejected without
writing a file. Existing files need `--force`.

```bash
apidiff capture --url http://localhost:8080/v3/api-docs --out baseline.json
```

## Output

`text` (default) groups findings into `breaking` and `non-breaking`, each with
the rule id, method and path, JSON pointer, and a one-line description. With no
changes it prints exactly `no contract changes`.

`json` emits one document with `base`, `revision`, `breaking`, `nonBreaking`
and `summary`.

`markdown` is a short ticket-ready report: a heading, the two source names, the
counts, and the breaking entries only.

## Development

See `AGENTS.md`. The suite runs through mise:

```bash
make check
# or
mise exec go -- go test -race ./...
```

Behavior lives in `docs/behavior.feature`; the technical brief in
`docs/context.md`.
