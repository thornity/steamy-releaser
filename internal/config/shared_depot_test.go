package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializeSharedAndSeparateDepots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "steam-release.yaml")
	options := InitOptions{Path: path, Name: "Game", Engine: "unity", AppID: 100,
		Targets: []InitTarget{
			{Name: "windows", DepotID: 102, Output: "builds/windows/Game.exe", BuildSetting: "StandaloneWindows64"},
			{Name: "linux", DepotID: 101, Output: "builds/linux/Game", BuildSetting: "StandaloneLinux64"},
			{Name: "macos", DepotID: 101, Output: "builds/macos/Game.app", BuildSetting: "StandaloneOSX"},
		},
	}
	document, err := Initialize(options)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range []*Document{document, loaded} {
		targets := doc.Config.Profiles[DefaultProfile].Targets
		for _, name := range []string{"linux", "macos"} {
			target := targets[name]
			if target.Content != "builds/"+name || len(target.Mappings) != 1 || target.Mappings[0].LocalPath != "*" || target.Mappings[0].DepotPath != name {
				t.Fatalf("unexpected %s shared-depot target: %#v", name, target)
			}
		}
		if len(targets["windows"].Mappings) != 0 || targets["windows"].DepotID != 102 {
			t.Fatalf("separate depot was changed: %#v", targets["windows"])
		}
	}
	contents, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(contents), "depot_path: linux") || !strings.Contains(string(contents), "depot_path: macos") {
		t.Fatalf("generated YAML does not show shared-depot destinations: %s, %v", contents, err)
	}
}

func TestInitializePreservesSharedDepotMappings(t *testing.T) {
	options := InitOptions{Path: filepath.Join(t.TempDir(), "steam-release.yaml"), Engine: "prebuilt", AppID: 100,
		Targets: []InitTarget{
			{Name: "linux", DepotID: 101, Content: "dist/linux", Mappings: []FileMapping{{LocalPath: "bin/*", DepotPath: "custom"}}},
			{Name: "windows", DepotID: 101, Content: "dist/windows"},
		},
	}
	document, _, err := Render(options)
	if err != nil {
		t.Fatal(err)
	}
	linux := document.Config.Profiles[DefaultProfile].Targets["linux"]
	if len(linux.Mappings) != 1 || linux.Mappings[0].LocalPath != "bin/*" || linux.Mappings[0].DepotPath != "custom" {
		t.Fatalf("explicit mapping changed: %#v", linux.Mappings)
	}
}

func TestSharedDepotRequiresUnambiguousDefaultMappings(t *testing.T) {
	for _, source := range []string{
		"app_id: 100\ntargets:\n  linux: {depot_id: 101, content: builds}\n  windows: {depot_id: 101, content: builds/windows}\n",
		"app_id: 100\ntargets:\n  linux: {depot_id: 101, content: builds/linux}\n  Linux: {depot_id: 101, content: builds/other}\n",
	} {
		path := filepath.Join(t.TempDir(), "steam-release.yaml")
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "define separate mappings") {
			t.Fatalf("ambiguous shared depot accepted: %v", err)
		}
	}
}

func TestInitializeRejectsAmbiguousSharedDepotDefaults(t *testing.T) {
	for _, targets := range [][]InitTarget{
		{{Name: "linux", DepotID: 101, Content: "builds"}, {Name: "windows", DepotID: 101, Content: "builds/windows"}},
		{{Name: "linux", DepotID: 101, Content: "builds/linux"}, {Name: "Linux", DepotID: 101, Content: "builds/other"}},
	} {
		options := InitOptions{Path: filepath.Join(t.TempDir(), "steam-release.yaml"), Engine: "prebuilt", AppID: 100, Targets: targets}
		if _, _, err := Render(options); err == nil || !strings.Contains(err.Error(), "define separate mappings") {
			t.Fatalf("initialized an ambiguous shared depot: %v", err)
		}
	}
}
