package appgen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerate_WritesRoutesFile(t *testing.T) {
	root := t.TempDir()
	copyDir(t, "testdata/repo", root)

	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, "_routes.py"))
	if err != nil {
		t.Fatalf("reading generated _routes.py: %v", err)
	}
	want, err := os.ReadFile("testdata/repo/_routes.py.golden")
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("generated _routes.py mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
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
		if rel == "." || rel == "_routes.py.golden" {
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
