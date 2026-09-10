# enigma-cli

**A fast, dependency-free replacement for Django's `makeurls`/dev-server
commands — written in Go so it never has to load Python to answer
questions about your Python code.**

## The problem

If you generate your Django `urlpatterns` dynamically (scanning a
`modules/<app>/api/<version>/` tree for view files, say), the usual way to
check "does this file define a view?" is to `import_module()` it and see.
That means every route-discovery pass pays the cost of importing every
candidate file — models, serializers, middleware, the works — even for
files you don't end up routing to. Do that on every dev-server reload, and
your edit-save-see-it loop gets slow.

## The fix

enigma-cli answers "does this file define an `APIView` class?" with a
regex over the raw source instead of an import. No Python interpreter, no
import graph, no Django — just reading files off disk. That's the whole
trick, and it's what makes everything else here possible:

- **Two speeds, your choice.** `--dev` switches the generated routes from
  **Eager** (every view resolved once, at import time — what production
  wants) to **Lazy** (each view resolved on first request — what a fast
  local reload loop wants).
- **The output has no runtime dependency on this tool.** Once the files are
  generated, Django only ever imports plain Python. enigma-cli only needs
  to exist at generation time.
- **One watcher, not two.** `enigma-cli server` supervises your dev server
  directly, so Django's own autoreloader gets disabled instead of racing
  it.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/AlmostWorkingSystem/enigma-cli/main/install.sh | sh
```

Installs the latest release to `/usr/local/bin` (`linux/amd64`,
`linux/arm64`, or `darwin/arm64`). Pin a version or change the install
directory:

```bash
ENIGMA_CLI_VERSION=v0.1.0 BINDIR="$HOME/.local/bin" \
  curl -fsSL https://raw.githubusercontent.com/AlmostWorkingSystem/enigma-cli/main/install.sh | sh
```

## Quickstart

```bash
cd your-django-project

# Generate _enigma.py and _routes.py once.
enigma-cli makeurls

# Or run the dev loop: generates once, starts `./manage.py runsslserver`,
# regenerates + restarts on relevant file changes, with fast Lazy-mode routes.
enigma-cli server --dev 0:8000
```

Then wire the two generated files into your settings and URLconf — see
[Integrating with your project](#integrating-with-your-project) below.

## Does your project fit?

enigma-cli expects a Django project shaped like this:

```
your-project/
├── enigma-config.yaml
└── modules/
    └── <module>/
        ├── apps.py                # optional: label = "..."
        ├── api/<version>/         # standalone module: api/ lives here
        │   └── ....py             # each defines `class APIView`
        └── <submodule>/           # OR: non-standalone module has submodules
            ├── apps.py
            └── api/<version>/...
```

- `enigma-config.yaml` declares each top-level module, whether it's
  `standalone` or has named submodules, and which API versions it serves.
- A module/submodule's `apps.py` may set `label = "..."`, used as its URL
  prefix. If absent, the label falls back to the last dotted segment of
  that file's `name = "modules.x.y"` (matching Django's own
  `AppConfig.label` default).
- Any file under `api/<version>/` defining `class APIView` becomes a
  route. `_`-prefixed files/directories (except `__init__.py`) are
  skipped. A directory/file literally named `<type:name>` (e.g.
  `<str:identifier>`) becomes a dynamic URL segment. A module-level
  `url_prefix`/`url_name` in a view file overrides the default
  segment/name.

If that's not your project's shape, this tool isn't a fit yet.

## CLI reference

Both subcommands take `--root` (default `.`).

```bash
enigma-cli makeurls [--root path] [--dev]
enigma-cli server [--root path] [--dev] [--skip-checks] <addr>
enigma-cli version   # or --version / -v
```

| Flag | Effect |
|---|---|
| `--dev` | Generate Lazy-mode routes (fast boot, per-route import on first use) instead of the Eager default (import-time resolution) |
| `--skip-checks` | Forwarded to the supervised `runsslserver`, skipping Django's system-check pass |
| `--root` | Path to the Django project root (default `.`) |

`server` also passes `--noreload` to the supervised `runsslserver` — its
own watcher already handles restarts, so Django's built-in reloader is
redundant. Its file-watching: any `*.py`/`*.html`/`.env` change (excluding
the two generated files and anything under `__pycache__`) restarts the
server; a narrower Create/Remove/Rename of a `.py` file under an `api/`
directory also regenerates the routes first. A newly created, empty API
file gets scaffolded with view boilerplate.

## Integrating with your project

Two files get generated, each with a different job:

- **`_enigma.py`** — `INSTALLED_APPS` and `ENIGMA_CONFIG`, with zero Django
  imports. Read this in `settings.py`, since Django needs `INSTALLED_APPS`
  before its app registry exists:

  ```python
  # settings.py
  from importlib import import_module

  _enigma = import_module("_enigma")

  INSTALLED_APPS = [
      "django.contrib.admin",
      "django.contrib.auth",
      # ...your other fixed apps...
  ] + _enigma.INSTALLED_APPS

  ENIGMA_CONFIG = _enigma.ENIGMA_CONFIG  # optional, if you need it at runtime
  ```

- **`_routes.py`** — `urlpatterns`. Read this in your root URLconf, which
  Django only ever imports after the app registry is already populated:

  ```python
  # urls.py
  from importlib import import_module

  urlpatterns = [
      # ...your own fixed URLs (admin, health checks, schema, etc.)...
  ] + import_module("_routes").urlpatterns
  ```

Add both generated files to your `.gitignore` — they're build artifacts,
regenerated by `enigma-cli makeurls`/`enigma-cli server`.

## Releases

Tags on `main` matching `v*` (SemVer: `vMAJOR.MINOR.PATCH`) trigger a
GitHub Actions build ([GoReleaser](https://goreleaser.com)) that publishes
binaries for `linux/amd64`, `linux/arm64`, and `darwin/arm64` to a GitHub
Release. Branch pushes never publish a release — only a tag push does.

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Development

Requires Go 1.27+.

```bash
git clone https://github.com/AlmostWorkingSystem/enigma-cli
cd enigma-cli
go build -o bin/enigma-cli ./cmd/enigma-cli
go test ./...
```

| Package | Responsibility |
|---|---|
| `internal/enigmaconfig` | Parses `enigma-config.yaml` into an order-preserving `Config` |
| `internal/pyscan` | Regex-based static scanning of Python source — never imports/executes it |
| `internal/routescan` | Walks the `modules/` tree into a flat route list |
| `internal/routegen` | Renders `_enigma.py` and `_routes.py` (Eager/Lazy) |
| `internal/apitemplate` | Boilerplate scaffolded into a newly created, empty API file |
| `internal/appgen` | Composes the packages above into the `makeurls` pipeline |
| `internal/watch` | fsnotify-based debounced file watcher |
| `internal/supervisor` | Child-process lifecycle for the supervised `runsslserver` |
| `cmd/enigma-cli` | CLI entrypoint (`makeurls`, `server`, `version`) |

There's also a read-only integration test that runs the real pipeline
against an actual Django project matching the layout above, and diffs the
result against that project's own existing `_routes.py`:

```bash
go test -tags integration ./internal/integration/...
```

It looks for such a project as a sibling directory by default; point
`ENIGMA_CLI_TEST_ROOT` elsewhere if needed. Read-only — it never writes to
the project it's pointed at.

Before committing:

```bash
go vet ./...
gofmt -l .    # should print nothing
go test ./...
```

Changes to `internal/routegen`'s output format are breaking changes for
whatever consumes the generated files — keep them deliberate.
