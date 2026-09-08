package pyscan

import "testing"

func TestScanAppLabel_ExplicitLabel(t *testing.T) {
	label, err := ScanAppLabel("testdata/apps_with_label.py")
	if err != nil {
		t.Fatalf("ScanAppLabel: %v", err)
	}
	if label != "hr" {
		t.Fatalf("want hr, got %s", label)
	}
}

func TestScanAppLabel_DefaultsToLastNameSegment(t *testing.T) {
	label, err := ScanAppLabel("testdata/apps_without_label.py")
	if err != nil {
		t.Fatalf("ScanAppLabel: %v", err)
	}
	if label != "core" {
		t.Fatalf("want core, got %s", label)
	}
}

func TestScanViewFile_DetectsAPIView(t *testing.T) {
	info, err := ScanViewFile("testdata/view_with_apiview.py")
	if err != nil {
		t.Fatalf("ScanViewFile: %v", err)
	}
	if !info.HasAPIView {
		t.Fatalf("want HasAPIView true")
	}
	if info.URLPrefix != nil || info.URLName != nil {
		t.Fatalf("want no overrides, got %+v", info)
	}
}

func TestScanViewFile_ReadsURLName(t *testing.T) {
	info, err := ScanViewFile("testdata/view_with_overrides.py")
	if err != nil {
		t.Fatalf("ScanViewFile: %v", err)
	}
	if info.URLName == nil || *info.URLName != "employee-from-aadhaar-verify" {
		t.Fatalf("want url_name override, got %+v", info)
	}
}

func TestScanViewFile_NoAPIView(t *testing.T) {
	info, err := ScanViewFile("testdata/not_a_view.py")
	if err != nil {
		t.Fatalf("ScanViewFile: %v", err)
	}
	if info.HasAPIView {
		t.Fatalf("want HasAPIView false")
	}
}
