package appgen

import (
	"os"
	"path/filepath"
	"time"

	"github.com/AlmostWorkingSystem/django-filerouter/internal/enigmaconfig"
	"github.com/AlmostWorkingSystem/django-filerouter/internal/routegen"
	"github.com/AlmostWorkingSystem/django-filerouter/internal/routescan"
)

// Generate runs the full makeurls pipeline against root and writes two
// files, returning how long it took (surfaced by the CLI, same as the
// Python command's own timing print):
//
//   - <root>/_enigma.py — INSTALLED_APPS + ENIGMA_CONFIG only, zero Django
//     imports. Read by kit/conf/__init__.py's initialize_conf() *before*
//     django.setup() runs, replacing what used to be computed on every
//     process start by re-parsing enigma-config.yaml and re-walking
//     modules/ (kit/conf/parser.py).
//   - <root>/_routes.py — urlpatterns, in the given routegen.Mode. Read by
//     config/urls.py *after* django.setup() has populated the app
//     registry — Eager mode's static imports (and Lazy mode's real
//     per-route imports once triggered) reach real Django model classes
//     transitively, which isn't safe any earlier.
//
// These two are deliberately separate files, not one: see
// routegen.RenderConfig's doc comment for why merging them crashes with
// AppRegistryNotReady.
func Generate(root string, mode routegen.Mode) (time.Duration, error) {
	start := time.Now()

	cfg, err := enigmaconfig.Load(root)
	if err != nil {
		return 0, err
	}
	routes, err := routescan.Scan(root, cfg)
	if err != nil {
		return 0, err
	}

	configContent := routegen.RenderConfig(cfg)
	if err := os.WriteFile(filepath.Join(root, "_enigma.py"), []byte(configContent), 0o644); err != nil {
		return 0, err
	}

	routesContent := routegen.RenderRoutes(routes, mode)
	if err := os.WriteFile(filepath.Join(root, "_routes.py"), []byte(routesContent), 0o644); err != nil {
		return 0, err
	}

	return time.Since(start), nil
}
