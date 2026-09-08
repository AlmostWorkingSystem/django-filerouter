//go:build integration

package integration

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/AlmostWorkingSystem/enigma-cli/internal/enigmaconfig"
	"github.com/AlmostWorkingSystem/enigma-cli/internal/routegen"
	"github.com/AlmostWorkingSystem/enigma-cli/internal/routescan"
)

// TestMakeURLsMatchesPythonOutput compares enigma-cli's generated
// _routes.py against the real camera_infra checkout's existing,
// Python-generated one. It never writes anything — pure read + diff.
//
// Route/import order *within* a given api version must match exactly (it's
// fully determined by the yaml and the filesystem tree). The outer
// api-version grouping (all v1 before all v0 in the current file) is not
// asserted: it comes from Python's os.walk(config/api), a raw filesystem
// listing order with no defined contract — see the design doc's Testing
// section. So this test compares import lines and path(...) lines as sets.
func TestMakeURLsMatchesPythonOutput(t *testing.T) {
	root := os.Getenv("ENIGMA_CLI_TEST_ROOT")
	if root == "" {
		root = "../../../camera_infra"
	}

	wantBytes, err := os.ReadFile(root + "/_routes.py")
	if err != nil {
		t.Fatalf("reading existing _routes.py (set ENIGMA_CLI_TEST_ROOT if camera_infra isn't a sibling): %v", err)
	}

	cfg, err := enigmaconfig.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	routes, err := routescan.Scan(root, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	got := routegen.Render(routes)

	assertSameLineSet(t, "import", importLines(got), importLines(string(wantBytes)))
	assertSameLineSet(t, "path(...)", pathLines(got), pathLines(string(wantBytes)))
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

func assertSameLineSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)

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
		t.Fatalf("%s line sets differ.\nmissing (%d): %v\nextra (%d): %v", label, len(missing), missing, len(extra), extra)
	}
}
