package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thornity/steam-release-tool/internal/config"
	"github.com/thornity/steam-release-tool/internal/release"
	"github.com/thornity/steam-release-tool/internal/steam"
)

func TestInitializedSharedDepotShipsAllTargets(t *testing.T) {
	root := t.TempDir()
	t.Setenv("STEAM_RELEASER_CONFIG_DIR", filepath.Join(t.TempDir(), "account"))
	steamCMD := filepath.Join(root, "steamcmd")
	if err := os.WriteFile(steamCMD, []byte("fake"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STEAMCMD_BIN", steamCMD)
	var targets []config.InitTarget
	for _, name := range []string{"linux", "macos", "windows"} {
		depot := uint64(101)
		if name == "windows" {
			depot = 102
		}
		content := filepath.Join(root, "builds", name)
		if err := os.MkdirAll(content, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(content, "Game"), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		targets = append(targets, config.InitTarget{Name: name, DepotID: depot, Content: filepath.Join("builds", name)})
	}
	path := filepath.Join(root, "steam-release.yaml")
	if _, err := config.Initialize(config.InitOptions{Path: path, Engine: "prebuilt", AppID: 100, Targets: targets}); err != nil {
		t.Fatal(err)
	}
	svc, err := Load(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{run: func(_ context.Context, _ release.CommandPlan) (release.RunResult, error) {
		return release.RunResult{Output: "Successfully finished AppID 100 build preview."}, nil
	}}
	svc.Runner = runner
	upload := steam.UploadOptions{Username: "builder", Preview: true}
	if _, err := svc.Ship(context.Background(), "", RunOptions{Targets: []string{"linux"}}, upload); err == nil || !strings.Contains(err.Error(), "select all targets") {
		t.Fatalf("partial shared depot upload was accepted: %v", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("partial selection started a process: %#v", runner.commands)
	}
	if _, err := svc.Plan("", RunOptions{Targets: []string{"windows"}}, upload); err != nil {
		t.Fatalf("separate depot could not be selected: %v", err)
	}
	result, err := svc.Ship(context.Background(), "", RunOptions{}, upload)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Scripts.DepotPaths) != 2 || len(runner.commands) != 1 {
		t.Fatalf("unexpected upload: %#v, commands %#v", result.Scripts, runner.commands)
	}
	depot, err := os.ReadFile(result.Scripts.DepotPaths[101])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"LocalPath" "linux/*"`, `"DepotPath" "linux"`, `"LocalPath" "macos/*"`, `"DepotPath" "macos"`} {
		if !strings.Contains(string(depot), want) {
			t.Fatalf("shared depot is missing %s: %s", want, depot)
		}
	}
}
