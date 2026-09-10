package enigmaconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Submodule struct {
	APIVersions []string
}

type SubmoduleEntry struct {
	Name      string
	Submodule Submodule
}

type ModuleSettings struct {
	Standalone bool
}

type Module struct {
	APIVersions []string
	Submodules  []SubmoduleEntry
	Settings    ModuleSettings
}

type ModuleEntry struct {
	Name   string
	Module Module
}

type Config struct {
	Modules []ModuleEntry
}

// DefaultFileName is the config file name the Python ConfigParser always
// resolves relative to the Django project root.
const DefaultFileName = "enigma-config.yaml"

// Load reads <root>/<fileName>. An empty fileName falls back to
// DefaultFileName.
func Load(root, fileName string) (*Config, error) {
	if fileName == "" {
		fileName = DefaultFileName
	}
	return loadFile(filepath.Join(root, fileName))
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	var modulesNode *yaml.Node
	if len(doc.Content) > 0 {
		modulesNode = mappingValue(doc.Content[0], "modules")
	}
	if modulesNode == nil {
		modulesNode = &yaml.Node{Kind: yaml.MappingNode}
	}
	if err := applyCoreDefaults(modulesNode); err != nil {
		return nil, err
	}

	var entries []ModuleEntry
	for i := 0; i < len(modulesNode.Content); i += 2 {
		name := modulesNode.Content[i].Value
		mod, err := parseModule(modulesNode.Content[i+1])
		if err != nil {
			return nil, fmt.Errorf("module %q: %w", name, err)
		}
		entries = append(entries, ModuleEntry{Name: name, Module: mod})
	}
	return &Config{Modules: entries}, nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func parseModule(node *yaml.Node) (Module, error) {
	var m Module
	if v := mappingValue(node, "api_versions"); v != nil {
		m.APIVersions = scalarValues(v)
	}
	if v := mappingValue(node, "settings"); v != nil {
		if s := mappingValue(v, "standalone"); s != nil {
			standalone, err := strconv.ParseBool(s.Value)
			if err != nil {
				return Module{}, fmt.Errorf("settings.standalone: invalid boolean %q: %w", s.Value, err)
			}
			m.Settings.Standalone = standalone
		}
	}
	if v := mappingValue(node, "submodules"); v != nil {
		for i := 0; i < len(v.Content); i += 2 {
			m.Submodules = append(m.Submodules, SubmoduleEntry{
				Name:      v.Content[i].Value,
				Submodule: parseSubmodule(v.Content[i+1]),
			})
		}
	}
	return m, nil
}

func parseSubmodule(node *yaml.Node) Submodule {
	var s Submodule
	if v := mappingValue(node, "api_versions"); v != nil {
		s.APIVersions = scalarValues(v)
	}
	return s
}

func scalarValues(seq *yaml.Node) []string {
	out := make([]string, 0, len(seq.Content))
	for _, item := range seq.Content {
		out = append(out, item.Value)
	}
	return out
}

// applyCoreDefaults ports kit/conf/config.py's DEFAULT_MODULES merged via
// deep_merge, narrowed to the one entry that constant actually contains
// today: core's api_versions gains "v1" if missing (deduped, so a config
// that already lists v1 doesn't cause routescan to visit it twice — the
// Python original tolerates the duplicate only because dict-based
// setdefault calls downstream happen to be idempotent; a literal port
// without dedup would double-emit every core route in Go), and
// settings.standalone defaults to true, raising a conflict error only if
// the config explicitly sets it to false (mirrors deep_merge's scalar
// conflict check: default true != explicit false).
func applyCoreDefaults(modulesNode *yaml.Node) error {
	for i := 0; i < len(modulesNode.Content); i += 2 {
		if modulesNode.Content[i].Value != "core" {
			continue
		}
		coreNode := modulesNode.Content[i+1]

		apiVersions := mappingValue(coreNode, "api_versions")
		if apiVersions == nil {
			apiVersions = &yaml.Node{Kind: yaml.SequenceNode}
			coreNode.Content = append(coreNode.Content, strNode("api_versions"), apiVersions)
		}
		if !containsScalar(apiVersions, "v1") {
			apiVersions.Content = append(apiVersions.Content, strNode("v1"))
		}

		settings := mappingValue(coreNode, "settings")
		if settings == nil {
			settings = &yaml.Node{Kind: yaml.MappingNode}
			coreNode.Content = append(coreNode.Content, strNode("settings"), settings)
		}
		if standalone := mappingValue(settings, "standalone"); standalone != nil {
			v, err := strconv.ParseBool(standalone.Value)
			if err != nil {
				return fmt.Errorf("core.settings.standalone: invalid boolean %q: %w", standalone.Value, err)
			}
			if !v {
				return fmt.Errorf("conflict at core.settings.standalone")
			}
		} else {
			settings.Content = append(settings.Content, strNode("standalone"), boolNode(true))
		}
		return nil
	}

	// core absent entirely: prepend it, matching a dict where
	// DEFAULT_MODULES's "core" key was never overridden by the config file.
	modulesNode.Content = append([]*yaml.Node{
		strNode("core"),
		{
			Kind: yaml.MappingNode,
			Content: []*yaml.Node{
				strNode("api_versions"),
				{Kind: yaml.SequenceNode, Content: []*yaml.Node{strNode("v1")}},
				strNode("settings"),
				{Kind: yaml.MappingNode, Content: []*yaml.Node{strNode("standalone"), boolNode(true)}},
			},
		},
	}, modulesNode.Content...)
	return nil
}

func containsScalar(seq *yaml.Node, value string) bool {
	for _, item := range seq.Content {
		if item.Value == value {
			return true
		}
	}
	return false
}

func strNode(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

func boolNode(v bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(v)}
}
