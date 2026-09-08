package routescan

import (
	"sort"
	"testing"

	"github.com/AlmostWorkingSystem/enigma-cli/internal/enigmaconfig"
)

func TestScan(t *testing.T) {
	cfg := &enigmaconfig.Config{
		Modules: []enigmaconfig.ModuleEntry{
			{
				Name: "demo",
				Module: enigmaconfig.Module{
					APIVersions: []string{"v1"},
					Settings:    enigmaconfig.ModuleSettings{Standalone: true},
				},
			},
		},
	}

	routes, err := Scan("testdata/repo", cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	byURL := map[string]RouteEntry{}
	for _, r := range routes {
		byURL[r.URL] = r
	}

	// init, alpha, detail, widget: widget/ has a static sibling ("alpha.py")
	// next to the dynamic "<str:identifier>" dir, so the sort comparator is
	// actually exercised (see order assertion below).
	if len(routes) != 4 {
		t.Fatalf("want 4 routes (init, alpha, detail, widget), got %d: %+v", len(routes), routes)
	}

	init, ok := byURL["v1/demo/"]
	if !ok {
		t.Fatalf("missing init route, got %v", keys(byURL))
	}
	if init.ModulePath != "modules.demo.api.v1" || init.IsDynamic {
		t.Fatalf("bad init route: %+v", init)
	}

	widget, ok := byURL["v1/demo/widget/"]
	if !ok {
		t.Fatalf("missing widget route, got %v", keys(byURL))
	}
	if widget.ModulePath != "modules.demo.api.v1.widget" || widget.Name != "widget-list" {
		t.Fatalf("bad widget route: %+v", widget)
	}

	alpha, ok := byURL["v1/demo/widget/alpha/"]
	if !ok {
		t.Fatalf("missing alpha route, got %v", keys(byURL))
	}
	if alpha.ModulePath != "modules.demo.api.v1.widget.alpha" || alpha.IsDynamic {
		t.Fatalf("bad alpha route: %+v", alpha)
	}

	detail, ok := byURL["v1/demo/widget/<str:identifier>/detail/"]
	if !ok {
		t.Fatalf("missing detail route, got %v", keys(byURL))
	}
	if !detail.IsDynamic {
		t.Fatalf("want detail route dynamic: %+v", detail)
	}
	if detail.ModulePath != "modules.demo.api.v1.widget.<str:identifier>.detail" {
		t.Fatalf("bad detail module path: %s", detail.ModulePath)
	}

	// Order assertion: "alpha.py" (static) and "<str:identifier>" (dynamic)
	// are siblings under widget/, so the static-before-dynamic comparator
	// must place alpha's route ahead of detail's route in the returned
	// slice. Asserting only via the byURL map (built above) would discard
	// this ordering entirely, so check the slice indices directly.
	alphaIdx := indexOfURL(routes, "v1/demo/widget/alpha/")
	detailIdx := indexOfURL(routes, "v1/demo/widget/<str:identifier>/detail/")
	if alphaIdx == -1 || detailIdx == -1 {
		t.Fatalf("could not locate alpha/detail in routes slice: %+v", routes)
	}
	if alphaIdx >= detailIdx {
		t.Fatalf("want static route (alpha, index %d) before dynamic sibling (detail, index %d): %+v", alphaIdx, detailIdx, routes)
	}
}

// TestScan_Submodules exercises the non-standalone branch of Scan, where a
// module declares submodules (Module.Submodules) instead of Settings.Standalone,
// mirroring the shape of real modules like hr/{core,roster}.
func TestScan_Submodules(t *testing.T) {
	cfg := &enigmaconfig.Config{
		Modules: []enigmaconfig.ModuleEntry{
			{
				Name: "demo2",
				Module: enigmaconfig.Module{
					Submodules: []enigmaconfig.SubmoduleEntry{
						{
							Name:      "core",
							Submodule: enigmaconfig.Submodule{APIVersions: []string{"v1"}},
						},
						{
							Name:      "roster",
							Submodule: enigmaconfig.Submodule{APIVersions: []string{"v1"}},
						},
					},
					Settings: enigmaconfig.ModuleSettings{Standalone: false},
				},
			},
		},
	}

	routes, err := Scan("testdata/repo", cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(routes) != 2 {
		t.Fatalf("want 2 routes (one per submodule), got %d: %+v", len(routes), routes)
	}

	byURL := map[string]RouteEntry{}
	for _, r := range routes {
		byURL[r.URL] = r
	}

	core, ok := byURL["v1/demo2_core/"]
	if !ok {
		t.Fatalf("missing core submodule route, got %v", keys(byURL))
	}
	if core.ModulePath != "modules.demo2.core.api.v1" || core.IsDynamic {
		t.Fatalf("bad core route: %+v", core)
	}

	roster, ok := byURL["v1/demo2_roster/"]
	if !ok {
		t.Fatalf("missing roster submodule route, got %v", keys(byURL))
	}
	if roster.ModulePath != "modules.demo2.roster.api.v1" || roster.IsDynamic {
		t.Fatalf("bad roster route: %+v", roster)
	}

	// Order assertion: submodules must be walked in cfg declaration order
	// (core before roster), matching Scan's doc comment on module ordering.
	coreIdx := indexOfURL(routes, "v1/demo2_core/")
	rosterIdx := indexOfURL(routes, "v1/demo2_roster/")
	if coreIdx >= rosterIdx {
		t.Fatalf("want core (index %d) before roster (index %d): %+v", coreIdx, rosterIdx, routes)
	}
}

func indexOfURL(routes []RouteEntry, url string) int {
	for i, r := range routes {
		if r.URL == url {
			return i
		}
	}
	return -1
}

func keys(m map[string]RouteEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
