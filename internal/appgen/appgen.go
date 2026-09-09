package appgen

import (
	"os"
	"path/filepath"
	"time"

	"github.com/AlmostWorkingSystem/enigma-cli/internal/enigmaconfig"
	"github.com/AlmostWorkingSystem/enigma-cli/internal/routegen"
	"github.com/AlmostWorkingSystem/enigma-cli/internal/routescan"
)

// Generate runs the full makeurls pipeline against root and writes
// <root>/_enigma.py in the given routegen.Mode, returning how long it took
// (surfaced by the CLI, same as the Python command's own timing print).
// _enigma.py unifies what used to be split across _routes.py (urlpatterns)
// and settings.ENIGMA_CONFIG/INSTALLED_APPS (computed at Django-startup
// time by kit/conf/parser.py) into one generated file, so Django's own
// bootstrap can just import plain data instead of re-parsing
// enigma-config.yaml and re-walking modules/ on every process start.
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
	content := routegen.Render(cfg, routes, mode)
	if err := os.WriteFile(filepath.Join(root, "_enigma.py"), []byte(content), 0o644); err != nil {
		return 0, err
	}

	return time.Since(start), nil
}
