package appgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlmostWorkingSystem/django-filerouter/internal/routegen"
)

func TestGenerate_WritesConfigAndRoutesFiles(t *testing.T) {
	cases := []struct {
		name         string
		mode         routegen.Mode
		routesGolden string
	}{
		{name: "Eager", mode: routegen.Eager, routesGolden: "testdata/repo/_routes.py.golden"},
		{name: "Lazy", mode: routegen.Lazy, routesGolden: "testdata/repo/_routes.py.lazy.golden"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			copyDir(t, "testdata/repo", root)

			if _, err := Generate(root, tc.mode, ""); err != nil {
				t.Fatalf("Generate: %v", err)
			}

			assertFileMatches(t, filepath.Join(root, "_enigma.py"), "testdata/repo/_enigma.py.golden")
			assertFileMatches(t, filepath.Join(root, "_routes.py"), tc.routesGolden)
		})
	}
}

func assertFileMatches(t *testing.T, gotPath, wantPath string) {
	t.Helper()

	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("reading generated %s: %v", gotPath, err)
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("reading golden file %s: %v", wantPath, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s mismatch.\n--- got ---\n%s\n--- want ---\n%s", gotPath, got, want)
	}
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." || rel == "_enigma.py.golden" || rel == "_routes.py.golden" || rel == "_routes.py.lazy.golden" {
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copyDir: %v", err)
	}
}
