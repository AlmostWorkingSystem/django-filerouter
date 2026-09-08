package routescan

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/AlmostWorkingSystem/enigma-cli/internal/enigmaconfig"
	"github.com/AlmostWorkingSystem/enigma-cli/internal/pyscan"
)

type RouteEntry struct {
	URL        string
	ModulePath string
	Name       string
	IsDynamic  bool
}

// dynamicSegmentRe mirrors Module.DYNAMIC_ROUTE_REGEX in kit/conf/config.py
// exactly: it's matched against the raw directory/file name (including the
// angle brackets), e.g. "<str:identifier>" satisfies `^[^:]+:[^:]+$` with
// the first group swallowing "<str" and the second "identifier>".
var dynamicSegmentRe = regexp.MustCompile(`^[^:]+:[^:]+$`)

// Scan walks the modules/ tree per Config and returns every discovered
// route. Module/submodule order in the result follows cfg.Modules order
// (the yaml's declaration order); within a single api-version tree, order
// is static-before-dynamic then alphabetical, matching _make_urls.
func Scan(root string, cfg *enigmaconfig.Config) ([]RouteEntry, error) {
	var out []RouteEntry
	for _, me := range cfg.Modules {
		if me.Module.Settings.Standalone {
			entries, err := scanApp(root, filepath.Join("modules", me.Name), "modules."+me.Name, me.Module.APIVersions)
			if err != nil {
				return nil, err
			}
			out = append(out, entries...)
			continue
		}
		for _, se := range me.Module.Submodules {
			appDir := filepath.Join("modules", me.Name, se.Name)
			dottedApp := fmt.Sprintf("modules.%s.%s", me.Name, se.Name)
			entries, err := scanApp(root, appDir, dottedApp, se.Submodule.APIVersions)
			if err != nil {
				return nil, err
			}
			out = append(out, entries...)
		}
	}
	return out, nil
}

func scanApp(root, appDir, dottedApp string, apiVersions []string) ([]RouteEntry, error) {
	var out []RouteEntry
	seen := map[string]bool{}
	for _, apiVersion := range apiVersions {
		if seen[apiVersion] {
			continue // config lists the same version twice; scan it once
		}
		seen[apiVersion] = true

		apiDir := filepath.Join(root, appDir, "api", apiVersion)
		if fi, err := os.Stat(apiDir); err != nil || !fi.IsDir() {
			continue // module has no api/<version> tree
		}

		label, err := pyscan.ScanAppLabel(filepath.Join(root, appDir, "apps.py"))
		if err != nil {
			return nil, err
		}

		leaves, err := makeURLs(apiDir, dottedApp+".api."+apiVersion, "")
		if err != nil {
			return nil, err
		}
		for _, lf := range leaves {
			out = append(out, RouteEntry{
				URL:        apiVersion + "/" + label + "/" + lf.relURL,
				ModulePath: lf.modulePath,
				Name:       lf.name,
				IsDynamic:  strings.Contains(lf.modulePath, "<"),
			})
		}
	}
	return out, nil
}

type leaf struct {
	relURL     string
	modulePath string
	name       string
}

// makeURLs ports Module._make_urls (kit/conf/config.py:124-192).
func makeURLs(dirFSPath, dottedModulePath, parentURL string) ([]leaf, error) {
	entries, err := os.ReadDir(dirFSPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dirFSPath, err)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		ni, nj := entries[i].Name(), entries[j].Name()
		di, dj := dynamicSegmentRe.MatchString(ni), dynamicSegmentRe.MatchString(nj)
		if di != dj {
			return !di // static sorts before dynamic
		}
		return ni < nj
	})

	var out []leaf
	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() {
			if strings.HasPrefix(name, "_") {
				continue
			}
			childParent := name
			if parentURL != "" {
				childParent = parentURL + "/" + name
			}
			children, err := makeURLs(filepath.Join(dirFSPath, name), dottedModulePath+"."+name, childParent)
			if err != nil {
				return nil, err
			}
			out = append(out, children...)
			continue
		}

		if !strings.HasSuffix(name, ".py") {
			continue
		}
		if strings.HasPrefix(name, "_") && name != "__init__.py" {
			continue
		}

		info, err := pyscan.ScanViewFile(filepath.Join(dirFSPath, name))
		if err != nil {
			return nil, err
		}
		if !info.HasAPIView {
			continue
		}

		segment := strings.TrimSuffix(name, ".py")
		switch {
		case info.URLPrefix != nil:
			segment = *info.URLPrefix
		case name == "__init__.py":
			segment = ""
		}

		full := segment
		if parentURL != "" {
			full = parentURL + "/" + segment
		}
		if full != "" && !strings.HasSuffix(full, "/") {
			full += "/"
		}

		modulePath := dottedModulePath
		if name != "__init__.py" {
			modulePath = dottedModulePath + "." + strings.TrimSuffix(name, ".py")
		}

		routeName := ""
		if info.URLName != nil {
			routeName = *info.URLName
		}

		out = append(out, leaf{relURL: full, modulePath: modulePath, name: routeName})
	}
	return out, nil
}
