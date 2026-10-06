# apply (search-replace-go)

A Go implementation of Aider's SEARCH/REPLACE ("editblock") diff format. It parses SEARCH/REPLACE blocks out of an LLM's response and applies them to files on disk, using the same matching strategies (exact match, whitespace-tolerant match, `...`-elided match, and fuzzy match) as the original [search-replace-py](https://github.com/marcius-llmus/search-replace-py) library.

## Repository layout

- `edit/` — the core Go package.
  - `parser.go` — finds and parses `<<<<<<< SEARCH` / `=======` / `>>>>>>> REPLACE` blocks, including filename discovery.
  - `apply.go` — applies parsed edits to files, with exact, whitespace-tolerant, and `...`-elided matching.
  - `fuzzy.go` — last-resort fuzzy matching and "did you mean" suggestions for failed matches.
  - `types.go` / `errors.go` — shared types and error types (`ParseError`, `PathEscapeError`, `ApplyError`).
- `cmd/apply/` — CLI entrypoint (`apply`) built on top of `edit`.
- `cache/` — a vendored copy of the upstream `search-replace-py` Python source, kept as the reference implementation this Go package was ported from.

## Build & install

```bash
make build     # builds ./apply
make install   # go install ./cmd/apply
```

## Usage

Pipe an LLM's diff response into `apply`. The diff must contain the filename on the line before each SEARCH block; `apply` takes no arguments other than `-h` / `--help` (running it without piped input also prints the help):

```bash
wl-paste | apply
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


## Testing

```bash
make test   # gofmt check, go vet, build, go test ./...
```
