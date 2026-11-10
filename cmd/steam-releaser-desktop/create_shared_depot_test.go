package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateConfigAllowsSharedAndSeparateDepots(t *testing.T) {
	root := t.TempDir()
	request := CreateConfigRequest{
		Path: filepath.Join(root, "steam-release.yaml"), Name: "Game", Engine: "unity", AppID: 100,
		Targets: []CreateTargetRequest{
			{Name: "linux", DepotID: 101, Path: "builds/linux/Game", BuildSetting: "StandaloneLinux64"},
			{Name: "macos", DepotID: 101, Path: "builds/macos/Game.app", BuildSetting: "StandaloneOSX"},
			{Name: "windows", DepotID: 102, Path: "builds/windows/Game.exe", BuildSetting: "StandaloneWindows64"},
		},
	}
	app := &DesktopApp{}
	preview, err := app.PreviewConfig(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"depot_path: linux", "depot_path: macos", "depot_id: 102"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview is missing %q:\n%s", want, preview)
		}
	}
	if _, err := app.CreateConfig(request); err != nil {
		t.Fatalf("creating the previewed configuration: %v", err)
	}
}
