# enigma-cli

A standalone Go CLI that replaces two Django management commands from
[camera_infra](https://github.com/AlmostWorkingSystem/camera_infra) — `makeurls`
and the dev `server` command's file-watch/restart loop — without needing
Python or Django to run. It exists because both commands import every API
view module just to answer questions ("what routes exist", "should I
restart") that don't actually require Django to be loaded: `enigma-cli`
answers them by statically scanning the filesystem instead, which is
significantly faster in both CI and local dev.

Design doc and implementation plan live in the `camera_infra` repo under
`docs/superpowers/specs/2026-09-08-enigma-cli-design.md` and
`docs/superpowers/plans/2026-09-08-enigma-cli.md`.

## Status

This is a standalone build, verified against the real `camera_infra`
checkout (see Testing below), but **not yet wired into `camera_infra`**
(justfile/Dockerfiles still call `./manage.py makeurls` / `./manage.py
server`) and has **no CI/release workflow yet**. Both are the next steps.

## Install / build

```bash
go build -o bin/enigma-cli ./cmd/enigma-cli
```

Requires Go 1.27+ (see `go.mod`).

## Usage

Both subcommands take `--root` (default `.`), the path to the Django
project root — same convention as running `./manage.py <command>` from
the repo root.

```bash
# One-shot: scan modules/, write _routes.py, print elapsed time.
enigma-cli makeurls [--root path]

# Dev loop: run makeurls once, then supervise `./manage.py runsslserver <addr>`,
# regenerating _routes.py and restarting it on relevant file changes.
enigma-cli server [--root path] <addr>
```

`server`'s file-watching mirrors the Python `WatchDogReloader` it replaces:
any `*.py`/`*.html`/`.env` file change (excluding `_routes.py` and
`__pycache__`) restarts the supervised server; a narrower
Create/Remove/Rename of a `.py` file under an `api/` directory additionally
triggers an in-process regenerate of `_routes.py` first, and a newly
created empty API file gets scaffolded with view boilerplate.

## Package layout

| Package | Responsibility |
|---|---|
| `internal/enigmaconfig` | Parses `enigma-config.yaml` into an order-preserving `Config`, replicating the real parser's `core`-module defaulting |
| `internal/pyscan` | Regex-based static scanning of Python source — `APIView` detection, `url_prefix`/`url_name` overrides, `apps.py` labels — never imports/executes Python |
| `internal/routescan` | Walks the `modules/` filesystem tree into a flat route list (port of `Module._make_urls`) |
| `internal/routegen` | Renders `_routes.py` from a route list, byte-compatible with the real Django-generated file |
| `internal/apitemplate` | Generates the view-file boilerplate scaffolded into a newly created, empty API file |
| `internal/appgen` | Composes `enigmaconfig` → `routescan` → `routegen` into the `makeurls` pipeline |
| `internal/watch` | fsnotify-based debounced file watcher |
| `internal/supervisor` | Owns the lifecycle (start/restart/stop) of the supervised `runsslserver` child process |
| `cmd/enigma-cli` | CLI entrypoint, wires up `makeurls` and `server` |

## Testing

```bash
go test ./...
```

There's also a read-only integration test that runs the real pipeline
against an actual `camera_infra` checkout and diffs the result against its
existing, Python-generated `_routes.py`:

```bash
go test -tags integration ./internal/integration/...
```

By default it looks for `camera_infra` as a sibling directory
(`../camera_infra`); override with `ENIGMA_CLI_TEST_ROOT` if it's checked
out elsewhere. This test only reads files — it never writes to the
`camera_infra` checkout.
