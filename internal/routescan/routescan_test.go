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

	if len(routes) != 3 {
		t.Fatalf("want 3 routes (init, widget, detail), got %d: %+v", len(routes), routes)
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
}

func keys(m map[string]RouteEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
