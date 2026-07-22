# search-replace-go

A Go implementation of Aider's SEARCH/REPLACE ("editblock") diff format. It parses SEARCH/REPLACE blocks out of an LLM's response and applies them to files on disk, using the same matching strategies (exact match, whitespace-tolerant match, `...`-elided match, and fuzzy match) as the original [search-replace-py](https://github.com/marcius-llmus/search-replace-py) library.

## Repository layout

- `searchreplace/` — the core Go package.
  - `parser.go` — finds and parses `<<<<<<< SEARCH` / `=======` / `>>>>>>> REPLACE` blocks, including filename discovery.
  - `apply.go` — applies parsed edits to files, with exact, whitespace-tolerant, and `...`-elided matching.
  - `fuzzy.go` — last-resort fuzzy matching and "did you mean" suggestions for failed matches.
  - `types.go` / `errors.go` — shared types and error types (`ParseError`, `PathEscapeError`, `ApplyError`).
- `cmd/apply/` — CLI entrypoint (`apply`) built on top of `searchreplace`.
- `scripts/` — a small Python CLI (`scripts/apply`) that does the same job using the original `search-replace-py` PyPI package, for environments where building the Go binary isn't convenient.
- `cache/` — a vendored copy of the upstream `search-replace-py` Python source, kept as the reference implementation this Go package was ported from.

## Build & install

```bash
make build     # builds ./apply
make install   # go install ./cmd/apply
```

## Usage

Pipe an LLM's diff response into `apply`. If the diff doesn't already contain a filename, pass it as an argument:

```bash
wl-paste | apply mathweb/flask/app.py
```

A SEARCH/REPLACE block looks like:

```
mathweb/flask/app.py
<<<<<<< SEARCH
from flask import Flask
=======
import math
from flask import Flask
>>>>>>> REPLACE
```

### Python CLI alternative

`scripts/apply` provides the same behavior via the `search-replace-py` package:

```bash
wl-paste | scripts/apply mathweb/flask/app.py
```

## Testing

```bash
make test   # gofmt check, go vet, build, go test ./...
```
