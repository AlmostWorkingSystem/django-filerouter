package pyscan

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type ViewInfo struct {
	HasAPIView bool
	URLPrefix  *string
	URLName    *string
}

var (
	apiViewRe   = regexp.MustCompile(`(?m)^class APIView\b`)
	urlPrefixRe = regexp.MustCompile(`(?m)^url_prefix\s*=\s*["']([^"']*)["']`)
	urlNameRe   = regexp.MustCompile(`(?m)^url_name\s*=\s*["']([^"']*)["']`)
	labelRe     = regexp.MustCompile(`(?m)^\s+label\s*=\s*["']([^"']*)["']`)
	appNameRe   = regexp.MustCompile(`(?m)^\s+name\s*=\s*["']([^"']*)["']`)
)

// ScanViewFile reads a candidate view file and reports whether it defines
// an APIView class and any url_prefix/url_name overrides, without
// executing or importing the file.
func ScanViewFile(path string) (ViewInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ViewInfo{}, fmt.Errorf("reading %s: %w", path, err)
	}
	info := ViewInfo{HasAPIView: apiViewRe.Match(data)}
	if m := urlPrefixRe.FindSubmatch(data); m != nil {
		v := string(m[1])
		info.URLPrefix = &v
	}
	if m := urlNameRe.FindSubmatch(data); m != nil {
		v := string(m[1])
		info.URLName = &v
	}
	return info, nil
}

// ScanAppLabel reads a module/submodule's apps.py and returns its Django
// app label: the explicit `label = "..."` assignment if present, else the
// last dotted segment of `name = "modules.x.y"` (Django's own AppConfig
// default).
func ScanAppLabel(appsPyPath string) (string, error) {
	data, err := os.ReadFile(appsPyPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", appsPyPath, err)
	}
	if m := labelRe.FindSubmatch(data); m != nil {
		return string(m[1]), nil
	}
	m := appNameRe.FindSubmatch(data)
	if m == nil {
		return "", fmt.Errorf("%s: no label or name assignment found", appsPyPath)
	}
	parts := strings.Split(string(m[1]), ".")
	return parts[len(parts)-1], nil
}
