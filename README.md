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

Verified against the real `camera_infra` checkout (see Testing below) and
wired into its `Justfile`'s `dev` recipe on the
`feature/enigma-cli-integration` branch there (binary currently built
locally and `docker cp`'d into the dev container — Dockerfile/CI wiring is
still a next step, along with a release workflow).

## Install / build

```bash
go build -o bin/enigma-cli ./cmd/enigma-cli
```

Requires Go 1.27+ (see `go.mod`).

## Usage

Both subcommands take `--root` (default `.`), the path to the Django
project root — same convention as running `./manage.py <command>` from
the repo root. Both also take `--dev`, which switches the generated route
format from **Eager** (default) to **Lazy**:

- **Eager** (production): every route gets a static import (or, for a
  `<type:name>`-shaped module path, a module-level resolve) at
  `_enigma.py` import time. Every view module imports up front — slower
  server boot, but nothing needs to resolve again per-request or for
  schema generation.
- **Lazy** (`--dev`, local iteration): every route delegates to a shared
  `_LazyAPIView` class that defers importing its view module until either
  a real request hits it or something (e.g. drf-spectacular's schema
  generation) introspects `.cls`. Much faster server boot, at the cost of
  a one-time import on first use per route instead of at startup.

```bash
# One-shot: scan modules/, write _enigma.py, print elapsed time.
enigma-cli makeurls [--root path] [--dev]

# Dev loop: run makeurls once, then supervise `./manage.py runsslserver <addr>`,
# regenerating _enigma.py and restarting it on relevant file changes.
enigma-cli server [--root path] [--dev] <addr>
```

`server`'s file-watching mirrors the Python `WatchDogReloader` it replaces:
any `*.py`/`*.html`/`.env` file change (excluding `_enigma.py` and
`__pycache__`) restarts the supervised server; a narrower
Create/Remove/Rename of a `.py` file under an `api/` directory additionally
triggers an in-process regenerate of `_enigma.py` first, and a newly
created empty API file gets scaffolded with view boilerplate.

`_enigma.py` unifies what the Python side used to split across `_routes.py`
(`urlpatterns`) and `settings.ENIGMA_CONFIG`/`INSTALLED_APPS` (computed at
Django-startup time by `kit/conf/parser.py`, re-parsing
`enigma-config.yaml` and re-walking `modules/` on every process start) into
one generated file Django just imports as plain data.

## Package layout

| Package | Responsibility |
|---|---|
| `internal/enigmaconfig` | Parses `enigma-config.yaml` into an order-preserving `Config`, replicating the real parser's `core`-module defaulting |
| `internal/pyscan` | Regex-based static scanning of Python source — `APIView` detection, `url_prefix`/`url_name` overrides, `apps.py` labels — never imports/executes Python |
| `internal/routescan` | Walks the `modules/` filesystem tree into a flat route list (port of `Module._make_urls`) |
| `internal/routegen` | Renders `_enigma.py` (`urlpatterns`, `ENIGMA_CONFIG`, `INSTALLED_APPS`) in Eager or Lazy mode |
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
existing, previously-generated `_enigma.py`:

```bash
go test -tags integration ./internal/integration/...
```

By default it looks for `camera_infra` as a sibling directory
(`../camera_infra`); override with `ENIGMA_CLI_TEST_ROOT` if it's checked
out elsewhere. This test only reads files — it never writes to the
`camera_infra` checkout.
