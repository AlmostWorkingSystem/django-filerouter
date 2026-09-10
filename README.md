# enigma-cli

A standalone Go CLI that generates Django URL configuration and manages a
Django dev server, without needing Python or Django loaded to do either.
Both are normally done by a Django management command that has to import
every API view module just to answer questions like "what routes exist" or
"should I restart" — this tool answers them by statically scanning the
filesystem instead, which is significantly faster in both CI and local dev.

## What it expects from your project

enigma-cli assumes a Django project laid out like this:

```
your-project/
├── enigma-config.yaml
└── modules/
    └── <module>/
        ├── apps.py                    # optional `label = "..."`
        ├── api/
        │   └── <version>/             # e.g. v1
        │       └── ... .py            # each defines a class APIView
        └── <submodule>/                # if the module isn't standalone
            ├── apps.py
            └── api/<version>/...
```

- **`enigma-config.yaml`** declares which top-level modules exist, whether
  each is `standalone` (its `api/` tree sits directly under the module) or
  has named submodules (each with their own `api/` tree), and which API
  versions each one serves.
- Each module/submodule's **`apps.py`** may declare `label = "..."` — used
  as the URL prefix for everything under it. If absent, the label falls
  back to the last dotted segment of that file's `name = "modules.x.y"`
  assignment (matching Django's own `AppConfig.label` default).
- Every file under an `api/<version>/` tree that defines a class named
  `APIView` becomes a route. A file or directory named starting with `_`
  (other than `__init__.py`) is skipped. A directory or file literally
  named `<type:name>` (e.g. `<str:identifier>`) becomes a dynamic URL
  segment. A module-level `url_prefix = "..."` or `url_name = "..."` in a
  view file overrides the segment/name Django would otherwise derive from
  its filename.

## Usage

Both subcommands take `--root` (default `.`), the path to the Django
project root. Both also take `--dev`, which switches the generated route
format from **Eager** (default) to **Lazy**:

- **Eager** (production): every route gets a static import (or, for a
  `<type:name>`-shaped module path, a module-level resolve) at
  `_routes.py` import time. Every view module imports up front — slower
  server boot, but nothing needs to resolve again per-request or for
  schema generation (e.g. drf-spectacular).
- **Lazy** (`--dev`, local iteration): every route delegates to a shared
  `_LazyAPIView` class that defers importing its view module until either
  a real request hits it or something introspects `.cls` (schema
  generation tools typically do). Much faster server boot, at the cost of
  a one-time import on first use per route instead of at startup.

```bash
# One-shot: scan modules/, write _enigma.py and _routes.py, print elapsed time.
enigma-cli makeurls [--root path] [--dev]

# Dev loop: run makeurls once, then supervise `./manage.py runsslserver <addr>`,
# regenerating both files and restarting it on relevant file changes.
enigma-cli server [--root path] [--dev] [--skip-checks] <addr>

# Print the version this binary was built at (a released binary reports its
# tag, e.g. "enigma-cli v0.1.0"; a local `go build` reports "enigma-cli dev").
enigma-cli version   # or --version / -v
```

`server` always passes `--noreload` to the supervised `runsslserver` —
enigma-cli's own watcher already restarts it on file changes, so Django's
built-in reloader would otherwise redundantly re-exec the process a second
time on every start. `--skip-checks` is forwarded through as-is, skipping
Django's system-check pass.

`server`'s file-watching: any `*.py`/`*.html`/`.env` file change (excluding
`_enigma.py`, `_routes.py`, and anything under `__pycache__`) restarts the
supervised server; a narrower Create/Remove/Rename of a `.py` file under an
`api/` directory additionally triggers an in-process regenerate of both
generated files first, and a newly created empty API file gets scaffolded
with view boilerplate.

## What gets generated

Two files, not one — they have mutually exclusive import-time constraints,
not just different content:

- **`_enigma.py`** — `INSTALLED_APPS` (plain dotted-string paths) and
  `ENIGMA_CONFIG` (two tiny dependency-free classes describing the
  module/submodule tree), with zero Django imports anywhere in it. This is
  meant to be imported *before* Django's `django.setup()` runs, since
  Django requires `INSTALLED_APPS` set before its app registry populates —
  something that has to happen before any Model class can safely be
  defined.
- **`_routes.py`** — `urlpatterns`, in Eager or Lazy mode. This is meant to
  be imported *after* `django.setup()` has populated the app registry —
  Eager mode's static imports (and any mode's view classes, once actually
  resolved) reach real Django model classes transitively, which isn't safe
  any earlier. An earlier version of this tool tried unifying both into
  one file and hit exactly that ordering conflict: importing the combined
  file for `INSTALLED_APPS` also pulled in Eager mode's view imports,
  which cascade into Django's own auth models defining a real Model class
  before the app registry existed to hold it — an immediate
  `AppRegistryNotReady` crash.

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

## Development

Requires Go 1.27+ (see `go.mod`).

```bash
git clone https://github.com/AlmostWorkingSystem/enigma-cli
cd enigma-cli
go build -o bin/enigma-cli ./cmd/enigma-cli
```

### Project layout

| Package | Responsibility |
|---|---|
| `internal/enigmaconfig` | Parses `enigma-config.yaml` into an order-preserving `Config` |
| `internal/pyscan` | Regex-based static scanning of Python source — `APIView` detection, `url_prefix`/`url_name` overrides, `apps.py` labels — never imports/executes Python |
| `internal/routescan` | Walks the `modules/` filesystem tree into a flat route list |
| `internal/routegen` | `RenderConfig` renders `_enigma.py`; `RenderRoutes` renders `_routes.py` in Eager or Lazy mode |
| `internal/apitemplate` | Generates the view-file boilerplate scaffolded into a newly created, empty API file |
| `internal/appgen` | Composes `enigmaconfig` → `routescan` → `routegen` into the `makeurls` pipeline |
| `internal/watch` | fsnotify-based debounced file watcher |
| `internal/supervisor` | Owns the lifecycle (start/restart/stop) of the supervised `runsslserver` child process |
| `cmd/enigma-cli` | CLI entrypoint, wires up `makeurls`, `server`, and `version` |

### Running tests

```bash
go test ./...
```

There's also a read-only integration test that runs the real pipeline
against an actual Django project matching the layout described above, and
diffs the result against that project's existing, previously-generated
`_routes.py` (plus a Lazy-mode round-trip check and an
`_enigma.py`-never-imports-Django check, both at the target project's full
scale):

```bash
go test -tags integration ./internal/integration/...
```

By default it looks for such a project as a sibling directory
(`../<project>` — see `internal/integration/makeurls_test.go` for the exact
default); override with `ENIGMA_CLI_TEST_ROOT` to point it elsewhere. This
test only reads files — it never writes to the project it's pointed at.

### Before committing

```bash
go vet ./...
gofmt -l .    # should print nothing
go test ./...
```

### Contributing

Open an issue or pull request. Keep changes to `internal/routegen`'s output
format byte-for-byte deliberate — anything that changes the shape of
`_enigma.py`/`_routes.py` is a breaking change for whatever's consuming
them.
