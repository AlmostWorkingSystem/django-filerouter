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
locally and `docker cp`'d into the dev container — Dockerfile/CI wiring to
fetch a released binary automatically is still a next step).

## Releases

Releases are cut from `main` via git tags, built by
[GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`) through
`.github/workflows/release.yml`. Pushing a tag matching `v*` builds
binaries for `linux/amd64`, `linux/arm64`, and `darwin/arm64` and attaches
them to a GitHub Release — no branch push ever triggers a release, only a
tag push does.

To cut one:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Version numbers follow [SemVer](https://semver.org):
`vMAJOR.MINOR.PATCH` — patch for fixes, minor for new features, major for
breaking changes.

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
  `_routes.py` import time. Every view module imports up front — slower
  server boot, but nothing needs to resolve again per-request or for
  schema generation.
- **Lazy** (`--dev`, local iteration): every route delegates to a shared
  `_LazyAPIView` class that defers importing its view module until either
  a real request hits it or something (e.g. drf-spectacular's schema
  generation) introspects `.cls`. Much faster server boot, at the cost of
  a one-time import on first use per route instead of at startup.

```bash
# One-shot: scan modules/, write _enigma.py and _routes.py, print elapsed time.
enigma-cli makeurls [--root path] [--dev]

# Dev loop: run makeurls once, then supervise `./manage.py runsslserver <addr>`,
# regenerating both files and restarting it on relevant file changes.
enigma-cli server [--root path] [--dev] [--skip-checks] <addr>
```

`server` always passes `--noreload` to the supervised `runsslserver` —
enigma-cli's own watcher already restarts it on file changes, so Django's
built-in reloader would otherwise redundantly re-exec the process a second
time on every start (measured: roughly doubles time-to-ready). `--skip-checks`
is forwarded through as-is, skipping Django's system-check pass.

`server`'s file-watching mirrors the Python `WatchDogReloader` it replaces:
any `*.py`/`*.html`/`.env` file change (excluding `_enigma.py`, `_routes.py`,
and `__pycache__`) restarts the supervised server; a narrower
Create/Remove/Rename of a `.py` file under an `api/` directory additionally
triggers an in-process regenerate of both generated files first, and a
newly created empty API file gets scaffolded with view boilerplate.

Two separate generated files, not one — they have mutually exclusive
import-time constraints, not just different content:

- **`_enigma.py`** — `INSTALLED_APPS` + `ENIGMA_CONFIG`, deliberately with
  zero Django imports anywhere in it. `kit/conf/__init__.py`'s
  `initialize_conf()` imports this *before* `django.setup()` runs (Django
  requires `INSTALLED_APPS` set before the app registry populates) —
  replacing what used to be recomputed on every process start by
  re-parsing `enigma-config.yaml` and re-walking `modules/`
  (`kit/conf/parser.py`).
- **`_routes.py`** — `urlpatterns`, in Eager or Lazy mode. `config/urls.py`
  imports this *after* `django.setup()` has populated the app registry —
  Eager mode's static imports (and any mode's view classes, once actually
  resolved) reach real Django model classes transitively, which isn't safe
  any earlier. An earlier version of this tool tried unifying both into
  one file and hit exactly that ordering conflict live: importing
  `_enigma.py` for `INSTALLED_APPS` also pulled in Eager mode's
  `kit.views.base` import, which cascades into `django.contrib.auth.models`
  defining a real Model class before the app registry existed to hold
  it — an immediate `AppRegistryNotReady` crash.

## Package layout

| Package | Responsibility |
|---|---|
| `internal/enigmaconfig` | Parses `enigma-config.yaml` into an order-preserving `Config`, replicating the real parser's `core`-module defaulting |
| `internal/pyscan` | Regex-based static scanning of Python source — `APIView` detection, `url_prefix`/`url_name` overrides, `apps.py` labels — never imports/executes Python |
| `internal/routescan` | Walks the `modules/` filesystem tree into a flat route list (port of `Module._make_urls`) |
| `internal/routegen` | `RenderConfig` renders `_enigma.py` (`INSTALLED_APPS`, `ENIGMA_CONFIG`); `RenderRoutes` renders `_routes.py` (`urlpatterns`) in Eager or Lazy mode |
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
existing, previously-generated `_routes.py` (plus a Lazy-mode round-trip
check and an `_enigma.py`-never-imports-Django check, both at the real
checkout's full scale):

```bash
go test -tags integration ./internal/integration/...
```

By default it looks for `camera_infra` as a sibling directory
(`../camera_infra`); override with `ENIGMA_CLI_TEST_ROOT` if it's checked
out elsewhere. This test only reads files — it never writes to the
`camera_infra` checkout.
