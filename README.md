# django-filerouter

**File-based routing for Django — create a file, get a route.**

Fed up of writing a view, then flipping over to `urls.py` to wire it up,
then forgetting you did and wondering ten minutes later why your new
endpoint 404s? Or a merge conflict on `urls.py` because two people added a
route in the same spot the same day? That's the entire class of problem
this tool exists to delete.

## The idea

Frontend frameworks settled this years ago: Next.js, Nuxt, SvelteKit,
Remix — your file layout *is* your routing table. Drop a file in the right
folder and the route exists. No central registry to hand-maintain, no
merge conflicts over who edited the routes file last, no risk of a route
existing in code but never actually being wired up.

django-filerouter brings that same convention to Django. You lay out view files
under a predictable folder structure; django-filerouter scans it and generates
the `urlpatterns` (and the small amount of settings wiring around it)
Django actually needs — as a standalone CLI, not a Django management
command.

## The structure it expects

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

- **`enigma-config.yaml`** declares each top-level module: whether it's
  `standalone` (its `api/` sits directly under the module) or has named
  submodules (each with their own `api/`), and which API versions it
  serves. Pass `--config <name>` to `makeurls`/`server` if you'd rather
  name this file something else.
- **`apps.py`** may set `label = "..."`, used as the module's URL prefix.
  If absent, it falls back to the last dotted segment of that file's
  `name = "modules.x.y"` — Django's own `AppConfig.label` default.
- **Any file under `api/<version>/`** that defines `class APIView` becomes
  a route, named after its path in the tree — no manual registration.
  `_`-prefixed files/directories (except `__init__.py`) are skipped. A
  file or directory literally named `<type:name>` (e.g. `<str:identifier>`)
  becomes a dynamic URL segment. A module-level `url_prefix`/`url_name`
  overrides the default.

If your project doesn't look like this, the convention isn't a fit yet —
this isn't a general-purpose Django URL scanner, it's one opinionated
shape.

## Why a CLI instead of a Django management command

Discovering "does this file define a view?" the usual way means
`import_module()`-ing every candidate file — there's no way to ask that
question without either parsing the source or actually running it, and
running it pays your whole app's import graph (models, serializers,
middleware) for files you might not even route to. django-filerouter answers it
with a regex over the raw source instead: no Python interpreter, no import
graph, just files on disk. That's what makes it cheap enough to re-run on
every save during local dev, and fast enough that a CI/build step barely
notices it.

It also means two more things fall out naturally:

- **Two speeds, your choice.** `--dev` switches the generated routes from
  **Eager** (every view resolved once, at import time — what production
  wants) to **Lazy** (each view resolved on first request — what a fast
  local reload loop wants).
- **Zero runtime dependency.** Once the files are generated, Django only
  ever imports plain Python — django-filerouter only has to exist at generation
  time, never while your app is actually serving requests.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/AlmostWorkingSystem/django-filerouter/main/install.sh | sh
```

Installs the latest release to `/usr/local/bin` (`linux/amd64`,
`linux/arm64`, or `darwin/arm64`). Pin a version or change the install
directory:

```bash
DJANGO_FILEROUTER_VERSION=v0.1.0 BINDIR="$HOME/.local/bin" \
  curl -fsSL https://raw.githubusercontent.com/AlmostWorkingSystem/django-filerouter/main/install.sh | sh
```

## Quickstart

```bash
cd your-django-project

# Generate _enigma.py and _routes.py once.
django-filerouter makeurls

# Or run the dev loop: generates once, starts `./manage.py runsslserver`,
# regenerates + restarts on relevant file changes, with fast Lazy-mode routes.
django-filerouter server --dev 0:8000
```

Then wire the two generated files in — see
[Integrating with your project](#integrating-with-your-project).

## CLI reference

Both subcommands take `--root` (default `.`) and `--config` (default
`enigma-config.yaml`).

```bash
django-filerouter makeurls [--root path] [--dev] [--config name]
django-filerouter server [--root path] [--dev] [--skip-checks] [--config name] <addr>
django-filerouter version   # or --version / -v
```

| Flag | Effect |
|---|---|
| `--dev` | Generate Lazy-mode routes (fast boot, per-route import on first use) instead of the Eager default (import-time resolution) |
| `--skip-checks` | Forwarded to the supervised `runsslserver`, skipping Django's system-check pass |
| `--root` | Path to the Django project root (default `.`) |
| `--config` | Name of the config file to read, relative to `--root` (default `enigma-config.yaml`) |

### Why `runsslserver`, not `runserver`

`server` supervises
[`runsslserver`](https://pypi.org/project/django-sslserver-v2/), not
Django's plain `runserver` — a lot of things browsers only allow over
HTTPS (secure cookies, WebAuthn/passkeys, service workers, geolocation,
clipboard access) either don't work at all or behave differently on plain
`http://localhost`, and testing that behavior against a mismatched
environment is its own source of "works locally, breaks in review" bugs.
`runsslserver` gives you a self-signed HTTPS dev server with no extra
setup, so what you're testing locally matches what actually ships.

### Why fsnotify, not Django's own reloader

`server` passes `--noreload` to the supervised process and does its own
file-watching instead, using [fsnotify](https://github.com/fsnotify/fsnotify)
— OS-level file system events (inotify on Linux, FSEvents on macOS), not
polling. Django's built-in `StatReloader` works by re-`stat()`-ing every
watched file on an interval and diffing mtimes; fsnotify gets told about a
change the moment the OS knows about it, no polling loop involved. Since
django-filerouter's watcher already owns "restart on change", leaving Django's own
reloader enabled on top would just mean two watchers doing the same job —
`--noreload` turns that off.

Any `*.py`/`*.html`/`.env` change (excluding the two generated files and
anything under `__pycache__`) restarts the server; a narrower
Create/Remove/Rename of a `.py` file under an `api/` directory also
regenerates the routes first. A newly created, empty API file gets
scaffolded with view boilerplate.

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
regenerated by `django-filerouter makeurls`/`django-filerouter server`.

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
git clone https://github.com/AlmostWorkingSystem/django-filerouter
cd django-filerouter
go build -o bin/django-filerouter ./cmd/django-filerouter
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
| `cmd/django-filerouter` | CLI entrypoint (`makeurls`, `server`, `version`) |

There's also a read-only integration test that runs the real pipeline
against an actual Django project matching the layout above, and diffs the
result against that project's own existing `_routes.py`:

```bash
go test -tags integration ./internal/integration/...
```

It looks for such a project as a sibling directory by default; point
`DJANGO_FILEROUTER_TEST_ROOT` elsewhere if needed. Read-only — it never writes to
the project it's pointed at.

Before committing:

```bash
go vet ./...
gofmt -l .    # should print nothing
go test ./...
```

Changes to `internal/routegen`'s output format are breaking changes for
whatever consumes the generated files — keep them deliberate.
