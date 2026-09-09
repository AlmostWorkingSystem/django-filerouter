//go:build integration

package integration

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/AlmostWorkingSystem/enigma-cli/internal/enigmaconfig"
	"github.com/AlmostWorkingSystem/enigma-cli/internal/routegen"
	"github.com/AlmostWorkingSystem/enigma-cli/internal/routescan"
)

// TestMakeURLsMatchesPythonOutput compares enigma-cli's Eager-mode
// RenderRoutes output against the real camera_infra checkout's existing,
// previously-generated _routes.py (Eager mode's import/path lines are
// still byte-compatible in shape with what the original Config.save_urls
// produced — see the design spec — so this keeps the original line-set
// comparison). It never writes anything — pure read + diff.
//
// Route/import order *within* a given api version must match exactly (it's
// fully determined by the yaml and the filesystem tree). The outer
// api-version grouping (all v1 before all v0 in the current file) is not
// asserted: it comes from Python's os.walk(config/api), a raw filesystem
// listing order with no defined contract — see the design doc's Testing
// section. So this test compares import lines and path(...) lines as sets.
func TestMakeURLsMatchesPythonOutput(t *testing.T) {
	root, _, routes := scanRealRepo(t)

	wantBytes, err := os.ReadFile(root + "/_routes.py")
	if err != nil {
		t.Fatalf("reading existing _routes.py (set ENIGMA_CLI_TEST_ROOT if camera_infra isn't a sibling, or generate one first with `enigma-cli makeurls`): %v", err)
	}

	got := routegen.RenderRoutes(routes, routegen.Eager)

	assertSameSet(t, "import", importLines(got), importLines(string(wantBytes)))
	assertSameSet(t, "path(...)", pathLines(got), pathLines(string(wantBytes)))
}

// TestLazyModeRoundTripsAllRealRoutes renders the same real route set in
// Lazy mode and confirms every module path routescan.Scan found appears
// exactly once inside a _LazyAPIView(...) call — at full production scale
// (~1000 routes), not just a small fixture. Lazy mode has no real-file
// counterpart to diff against (the committed _routes.py is Eager-format),
// so this is a self-consistency check instead.
func TestLazyModeRoundTripsAllRealRoutes(t *testing.T) {
	_, _, routes := scanRealRepo(t)

	got := routegen.RenderRoutes(routes, routegen.Lazy)

	want := make([]string, 0, len(routes))
	for _, r := range routes {
		want = append(want, r.ModulePath)
	}

	var gotModules []string
	for _, m := range lazyAPIViewModuleRe.FindAllStringSubmatch(got, -1) {
		gotModules = append(gotModules, m[1])
	}

	assertSameSet(t, "_LazyAPIView module path", gotModules, want)
}

// TestRenderConfigNeverImportsDjangoAtRealScale confirms RenderConfig's
// "zero Django imports" invariant holds against the real ~30-module
// enigma-config.yaml, not just a small fixture, and that every module the
// real config declares round-trips into INSTALLED_APPS.
func TestRenderConfigNeverImportsDjangoAtRealScale(t *testing.T) {
	_, cfg, _ := scanRealRepo(t)

	got := routegen.RenderConfig(cfg)

	for _, line := range strings.Split(got, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "from django") || strings.HasPrefix(trimmed, "import django") {
			t.Fatalf("_enigma.py must never import Django: %q", trimmed)
		}
	}

	var wantApps []string
	for _, me := range cfg.Modules {
		if me.Module.Settings.Standalone {
			wantApps = append(wantApps, "modules."+me.Name)
			continue
		}
		for _, se := range me.Module.Submodules {
			wantApps = append(wantApps, "modules."+me.Name+"."+se.Name)
		}
	}

	var gotApps []string
	for _, m := range installedAppRe.FindAllStringSubmatch(got, -1) {
		gotApps = append(gotApps, m[1])
	}

	assertSameSet(t, "INSTALLED_APPS entry", gotApps, wantApps)
}

func scanRealRepo(t *testing.T) (root string, cfg *enigmaconfig.Config, routes []routescan.RouteEntry) {
	t.Helper()

	root = os.Getenv("ENIGMA_CLI_TEST_ROOT")
	if root == "" {
		root = "../../../camera_infra"
	}

	cfg, err := enigmaconfig.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	routes, err = routescan.Scan(root, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return root, cfg, routes
}

func importLines(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "from ") && strings.Contains(line, "import APIView as ") {
			out = append(out, line)
		}
	}
	return out
}

func pathLines(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "path(") {
			out = append(out, trimmed)
		}
	}
	return out
}

var lazyAPIViewModuleRe = regexp.MustCompile(`_LazyAPIView\('([^']*)'\)`)
var installedAppRe = regexp.MustCompile(`(?m)^\s+"([^"]*)",$`)

func assertSameSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)

	if len(want) == 0 {
		t.Fatalf("%s: want set is empty — comparison would be vacuous", label)
	}

	gotSet := map[string]bool{}
	for _, l := range got {
		gotSet[l] = true
	}
	wantSet := map[string]bool{}
	for _, l := range want {
		wantSet[l] = true
	}

	var missing, extra []string
	for _, l := range want {
		if !gotSet[l] {
			missing = append(missing, l)
		}
	}
	for _, l := range got {
		if !wantSet[l] {
			extra = append(extra, l)
		}
	}
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("%s sets differ.\nmissing (%d): %v\nextra (%d): %v", label, len(missing), missing, len(extra), extra)
	}
	t.Logf("%s: %d entries matched", label, len(want))
}
