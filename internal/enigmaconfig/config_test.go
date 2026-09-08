package enigmaconfig

import "testing"

func TestLoad_PreservesDeclarationOrderAndAppliesCoreDefaults(t *testing.T) {
	cfg, err := Load("testdata")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Modules) != 2 {
		t.Fatalf("want 2 modules, got %d", len(cfg.Modules))
	}

	core := cfg.Modules[0]
	if core.Name != "core" {
		t.Fatalf("want core first, got %s", core.Name)
	}
	if len(core.Module.APIVersions) != 1 || core.Module.APIVersions[0] != "v1" {
		t.Fatalf("want deduped core api_versions [v1], got %v", core.Module.APIVersions)
	}
	if !core.Module.Settings.Standalone {
		t.Fatalf("want core standalone true")
	}

	hr := cfg.Modules[1]
	if hr.Name != "hr" {
		t.Fatalf("want hr second, got %s", hr.Name)
	}
	if hr.Module.Settings.Standalone {
		t.Fatalf("want hr standalone false (not set, no default applies to non-core modules)")
	}
	if len(hr.Module.Submodules) != 2 || hr.Module.Submodules[0].Name != "core" || hr.Module.Submodules[1].Name != "roster" {
		t.Fatalf("want submodules [core, roster] in declared order, got %+v", hr.Module.Submodules)
	}
}

func TestLoad_SynthesizesCoreWhenAbsent(t *testing.T) {
	cfg, err := loadFile("testdata/enigma-config-no-core.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Modules) != 2 {
		t.Fatalf("want synthetic core + hr, got %d modules", len(cfg.Modules))
	}
	if cfg.Modules[0].Name != "core" {
		t.Fatalf("want synthesized core prepended first, got %s", cfg.Modules[0].Name)
	}
	if !cfg.Modules[0].Module.Settings.Standalone {
		t.Fatalf("want synthesized core standalone true")
	}
}

func TestLoad_ConflictOnExplicitStandaloneFalse(t *testing.T) {
	_, err := loadFile("testdata/enigma-config-conflict.yaml")
	if err == nil {
		t.Fatalf("want conflict error, got nil")
	}
}

func TestLoad_MalformedStandaloneOnNonCoreModuleIsHardError(t *testing.T) {
	_, err := loadFile("testdata/enigma-config-malformed-standalone.yaml")
	if err == nil {
		t.Fatalf("want hard error for malformed (non-boolean) standalone value, got nil")
	}
}

func TestLoad_MalformedCoreStandaloneIsHardError(t *testing.T) {
	_, err := loadFile("testdata/enigma-config-malformed-core-standalone.yaml")
	if err == nil {
		t.Fatalf("want hard error for malformed (non-boolean) core.settings.standalone value, got nil")
	}
}

func TestLoad_EmptyConfigSynthesizesCore(t *testing.T) {
	cfg, err := loadFile("testdata/enigma-config-empty.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Modules) != 1 {
		t.Fatalf("want synthetic core module for empty config, got %d modules", len(cfg.Modules))
	}
	if cfg.Modules[0].Name != "core" {
		t.Fatalf("want core module, got %s", cfg.Modules[0].Name)
	}
	if !cfg.Modules[0].Module.Settings.Standalone {
		t.Fatalf("want core standalone true")
	}
	if len(cfg.Modules[0].Module.APIVersions) != 1 || cfg.Modules[0].Module.APIVersions[0] != "v1" {
		t.Fatalf("want core api_versions [v1], got %v", cfg.Modules[0].Module.APIVersions)
	}
}
