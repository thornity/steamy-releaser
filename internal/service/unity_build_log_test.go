package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/thornity/steam-release-tool/internal/config"
	"github.com/thornity/steam-release-tool/internal/release"
	"github.com/thornity/steam-release-tool/internal/steam"
)

func createUnityProjectMarkers(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"Assets", "ProjectSettings"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectSettings", "ProjectVersion.txt"), []byte("m_EditorVersion: 6000.3.16f1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestUnityIncrementalBuildWithUnchangedOutput(t *testing.T) {
	for _, tc := range []struct {
		name, output, buildTarget string
	}{
		{"linux", "build/linux/ReignOfArrows.x86_64", "StandaloneLinux64"},
		{"windows", "build/windows/ReignOfArrows.exe", "StandaloneWindows64"},
	} {
		for _, action := range []string{"build", "ship"} {
			t.Run(tc.name+"/"+action, func(t *testing.T) {
				root := t.TempDir()
				createUnityProjectMarkers(t, root)
				if err := os.MkdirAll(filepath.Join(root, filepath.Dir(tc.output)), 0o755); err != nil {
					t.Fatal(err)
				}
				output := filepath.Join(root, tc.output)
				if err := os.WriteFile(output, []byte("existing player"), 0o700); err != nil {
					t.Fatal(err)
				}
				tool := filepath.Join(root, "tool")
				if err := os.WriteFile(tool, []byte("tool"), 0o700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("STEAM_RELEASER_CONFIG_DIR", filepath.Join(t.TempDir(), "account"))
				document := testDocument(root, config.Engine{Type: "unity", Executable: tool}, config.Target{
					Platform: tc.name, DepotID: 101, Output: tc.output, Content: filepath.Dir(tc.output),
					Build: config.Build{Target: tc.buildTarget},
				})
				if err := os.WriteFile(filepath.Join(root, "license.txt"), []byte("license"), 0o600); err != nil {
					t.Fatal(err)
				}
				profile := document.Config.Profiles["release"]
				profile.PostBuild = []config.CopyRule{{From: "license.txt"}}
				document.Config.Profiles["release"] = profile
				document.Config.Tools.SteamCMD = tool
				var events []release.Event
				runner := &fakeRunner{run: func(_ context.Context, command release.CommandPlan) (release.RunResult, error) {
					if command.Label == "SteamCMD upload" {
						return release.RunResult{Output: "Successfully finished AppID 100 build preview."}, nil
					}
					index := slices.Index(command.Args, "-logFile")
					if index < 0 || index+1 >= len(command.Args) {
						return release.RunResult{}, fmt.Errorf("Unity log argument is missing")
					}
					return release.RunResult{}, os.WriteFile(command.Args[index+1], []byte("Not rebuilding Data files -- no changes\nBuild Finished, Result: Success.\n"), 0o600)
				}}
				svc := &Service{Document: document, Runner: runner, Reporter: release.ReporterFunc(func(event release.Event) {
					events = append(events, event)
				})}
				var err error
				if action == "ship" {
					_, err = svc.Ship(context.Background(), "release", RunOptions{CaptureFingerprints: true}, steam.UploadOptions{Username: "builder", Preview: true})
				} else {
					err = svc.Build(context.Background(), "release", RunOptions{CaptureFingerprints: true})
				}
				if err != nil {
					t.Fatalf("incremental Unity %s failed: %v", action, err)
				}
				if contents, err := os.ReadFile(output); err != nil || string(contents) != "existing player" {
					t.Fatalf("existing output changed: %q, %v", contents, err)
				}
				if contents, err := os.ReadFile(filepath.Join(root, filepath.Dir(tc.output), "license.txt")); err != nil || string(contents) != "license" {
					t.Fatalf("static post-build file missing: %q, %v", contents, err)
				}
				log := filepath.Join(root, ".steam-releaser", "logs", "release", "target.log")
				if contents, err := os.ReadFile(log); err != nil || !strings.Contains(string(contents), "Build Finished, Result: Success.") {
					t.Fatalf("Unity log was not preserved: %q, %v", contents, err)
				}
				var warned, completed bool
				for _, event := range events {
					warned = warned || event.Level == release.LevelWarning && strings.Contains(event.Message, "configured output did not change")
					completed = completed || event.Kind == "build-complete"
				}
				if !warned || !completed {
					t.Fatalf("missing incremental build warning or checkpoint: %#v", events)
				}
				wantCommands := 1
				if action == "ship" {
					wantCommands = 2
				}
				if len(runner.commands) != wantCommands {
					t.Fatalf("unexpected commands: %#v", runner.commands)
				}
			})
		}
	}
}

func TestUnityRequiresCurrentSuccessfulBuildLogForUnchangedOutput(t *testing.T) {
	for _, tc := range []struct {
		name, currentLog, wantError string
		commandFails                bool
	}{
		{name: "no current log", wantError: "no change was detected"},
		{name: "incomplete log", currentLog: "BuildPlayer: start building target 24\n", wantError: "no change was detected"},
		{name: "failure after success", currentLog: "Build Finished, Result: Success.\nBuild Finished, Result: Failure.\n", wantError: "no change was detected"},
		{name: "failed process", currentLog: "Build Finished, Result: Success.\n", wantError: "Unity exited unsuccessfully", commandFails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			createUnityProjectMarkers(t, root)
			for _, dir := range []string{"build", ".steam-releaser/logs/release"} {
				if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			output := filepath.Join(root, "build", "Game.exe")
			if err := os.WriteFile(output, []byte("old player"), 0o700); err != nil {
				t.Fatal(err)
			}
			oldLog := filepath.Join(root, ".steam-releaser/logs/release/target.log")
			if err := os.WriteFile(oldLog, []byte("Build Finished, Result: Success.\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			document := testDocument(root, config.Engine{Type: "unity"}, config.Target{
				DepotID: 101, Output: "build/Game.exe", Content: "build", Build: config.Build{Target: "StandaloneWindows64"},
			})
			runner := &fakeRunner{run: func(_ context.Context, command release.CommandPlan) (release.RunResult, error) {
				if tc.currentLog == "" {
					return release.RunResult{}, nil
				}
				index := slices.Index(command.Args, "-logFile")
				if err := os.WriteFile(command.Args[index+1], []byte(tc.currentLog), 0o600); err != nil {
					return release.RunResult{}, err
				}
				if tc.commandFails {
					return release.RunResult{}, errors.New("Unity exited unsuccessfully")
				}
				return release.RunResult{}, nil
			}}
			svc := &Service{Document: document, Runner: runner, Reporter: release.NopReporter{}}
			if err := svc.Build(context.Background(), "release", RunOptions{}); err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
			if tc.currentLog == "" {
				if contents, err := os.ReadFile(oldLog); err != nil || string(contents) != "Build Finished, Result: Success.\n" {
					t.Fatalf("prior log changed: %q, %v", contents, err)
				}
			}
		})
	}
}

func TestUnityBuildLogDoesNotFollowLinkedLog(t *testing.T) {
	directory := t.TempDir()
	destination := filepath.Join(directory, "target.log")
	stale := filepath.Join(directory, "stale.log")
	if err := os.WriteFile(stale, []byte("Build Finished, Result: Success.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command, log, err := prepareUnityBuildLog(release.CommandPlan{Args: []string{"-logFile", destination}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(stale, command.Args[1]); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if succeeded, err := log.finish(); err == nil || succeeded {
		t.Fatalf("linked log was trusted: succeeded=%v, err=%v", succeeded, err)
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("linked log replaced the diagnostic log: %v", err)
	}
}

func TestUnitySuccessLogCannotSupplyMissingOrEmptyOutput(t *testing.T) {
	for _, emptyDirectory := range []bool{false, true} {
		root := t.TempDir()
		createUnityProjectMarkers(t, root)
		if err := os.MkdirAll(filepath.Join(root, "build"), 0o755); err != nil {
			t.Fatal(err)
		}
		if emptyDirectory {
			if err := os.Mkdir(filepath.Join(root, "build/Game"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		document := testDocument(root, config.Engine{Type: "unity"}, config.Target{
			DepotID: 101, Output: "build/Game", Content: "build", Build: config.Build{Target: "StandaloneLinux64"},
		})
		runner := &fakeRunner{run: func(_ context.Context, command release.CommandPlan) (release.RunResult, error) {
			index := slices.Index(command.Args, "-logFile")
			return release.RunResult{}, os.WriteFile(command.Args[index+1], []byte("Build Finished, Result: Success.\n"), 0o600)
		}}
		svc := &Service{Document: document, Runner: runner, Reporter: release.NopReporter{}}
		if err := svc.Build(context.Background(), "release", RunOptions{}); err == nil {
			t.Fatalf("Unity success log accepted a missing or empty output (empty directory: %v)", emptyDirectory)
		}
	}
}

func TestUnitySuccessLogCannotAuthorizeStalePostBuildReplacement(t *testing.T) {
	root := t.TempDir()
	createUnityProjectMarkers(t, root)
	for name, contents := range map[string]string{"build/Game.exe": "current player", "old/Game.exe": "stale player"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	document := testDocument(root, config.Engine{Type: "unity"}, config.Target{
		DepotID: 101, Output: "build/Game.exe", Content: "build", Build: config.Build{Target: "StandaloneWindows64"},
	})
	profile := document.Config.Profiles["release"]
	profile.PostBuild = []config.CopyRule{{From: "old/Game.exe"}}
	document.Config.Profiles["release"] = profile
	runner := &fakeRunner{run: func(_ context.Context, command release.CommandPlan) (release.RunResult, error) {
		index := slices.Index(command.Args, "-logFile")
		return release.RunResult{}, os.WriteFile(command.Args[index+1], []byte("Build Finished, Result: Success.\n"), 0o600)
	}}
	svc := &Service{Document: document, Runner: runner, Reporter: release.NopReporter{}}
	if err := svc.Build(context.Background(), "release", RunOptions{}); err == nil {
		t.Fatal("Unity success allowed a stale post-build replacement")
	}
	if contents, err := os.ReadFile(filepath.Join(root, "build/Game.exe")); err != nil || string(contents) != "current player" {
		t.Fatalf("stale copy changed the player: %q, %v", contents, err)
	}
}
