package steam

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thornity/steam-release-tool/internal/config"
)

func TestSharedDepotRebasesDifferentContentDirectories(t *testing.T) {
	root := t.TempDir()
	nonRecursive := false
	document := &config.Document{Path: filepath.Join(root, "steam-release.yaml"), Config: config.Config{
		Profiles: map[string]config.Profile{"release": {AppID: 100, Targets: map[string]config.Target{
			"linux": {DepotID: 101, Content: "builds/linux", Exclusions: []string{"cache/*"}},
			"macos": {DepotID: 101, Content: "dist/macos", Mappings: []config.FileMapping{
				{LocalPath: "assets/*", DepotPath: "macos/data", Recursive: &nonRecursive},
			}, Exclusions: []string{"*.pdb"}},
			"windows": {DepotID: 102, Content: "builds/windows"},
		}}},
	}}
	if err := ValidateScriptSelection(document, "release", []string{"linux"}, ScriptOptions{}); err == nil || !strings.Contains(err.Error(), "select all targets") {
		t.Fatalf("partial shared-depot selection was accepted: %v", err)
	}
	scripts, err := Generate(document, "release", nil, ScriptOptions{Preview: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts.DepotPaths) != 2 {
		t.Fatalf("got depots %#v, want one shared and one separate", scripts.DepotPaths)
	}
	assertVDFContains(t, scripts.DepotPaths[101],
		`"ContentRoot" "`+root+`"`,
		`"LocalPath" "builds/linux/*"`, `"DepotPath" "linux"`,
		`"LocalPath" "dist/macos/assets/*"`, `"DepotPath" "macos/data"`, `"Recursive" "0"`,
		`"FileExclusion" "builds/linux/cache/*"`, `"FileExclusion" "dist/macos/*.pdb"`,
		`"FileExclusion" "*.gitkeep"`)
	assertVDFContains(t, scripts.DepotPaths[102],
		`"ContentRoot" "`+filepath.Join(root, "builds", "windows")+`"`, `"LocalPath" "*"`, `"DepotPath" "."`)
}

func TestSharedDepotRejectsUnrepresentableContentPathsBeforeWriting(t *testing.T) {
	for _, path := range []string{"builds/*special/linux", "builds/question?/linux"} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			document := &config.Document{Path: filepath.Join(root, "steam-release.yaml"), Config: config.Config{
				Profiles: map[string]config.Profile{"release": {AppID: 100, Targets: map[string]config.Target{
					"linux": {DepotID: 101, Content: path},
					"macos": {DepotID: 101, Content: "builds/macos"},
				}}},
			}}
			if err := ValidateScriptSelection(document, "release", nil, ScriptOptions{}); err == nil || !strings.Contains(err.Error(), "wildcard") {
				t.Fatalf("unsafe rebased path passed preflight: %v", err)
			}
			if _, err := Generate(document, "release", nil, ScriptOptions{}); err == nil {
				t.Fatal("unsafe rebased path generated scripts")
			}
			if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
				t.Fatalf("rejected input wrote files: %v, %v", entries, err)
			}
		})
	}
}
