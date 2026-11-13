package service

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/thornity/steam-release-tool/internal/release"
	"github.com/thornity/steam-release-tool/internal/safefile"
)

const unityLogTailLimit = 8 << 20

// unityBuildLog isolates each invocation so an old success line cannot make
// an unchanged output appear to have been built by the current command.
type unityBuildLog struct {
	directory   string
	name        string
	destination string
	identity    fs.FileInfo
	preserved   bool
}

func prepareUnityBuildLog(command release.CommandPlan) (release.CommandPlan, *unityBuildLog, error) {
	index := slices.Index(command.Args, "-logFile")
	if index < 0 || index+1 >= len(command.Args) {
		return command, nil, fmt.Errorf("Unity command has no managed log path")
	}
	destination := command.Args[index+1]
	directory, err := os.MkdirTemp(filepath.Dir(destination), "."+filepath.Base(destination)+"-")
	if err != nil {
		return command, nil, err
	}
	identity, err := os.Lstat(directory)
	if err != nil || !identity.IsDir() || !os.SameFile(identity, identity) {
		_ = os.Remove(directory)
		return command, nil, fmt.Errorf("cannot identify temporary Unity log directory")
	}
	log := &unityBuildLog{directory: directory, name: filepath.Base(destination), destination: destination, identity: identity}
	command.Args = slices.Clone(command.Args)
	command.Args[index+1] = filepath.Join(directory, log.name)
	return command, log, nil
}

// finish checks the current invocation's log and moves it to the usual log
// path for diagnostics. It leaves the temporary file in place if moving fails.
func (log *unityBuildLog) finish() (bool, error) {
	current, err := os.Lstat(log.directory)
	if err != nil || !current.IsDir() || !os.SameFile(log.identity, current) {
		return false, fmt.Errorf("Unity log directory changed: %s", log.directory)
	}
	root, err := os.OpenRoot(log.directory)
	if err != nil {
		return false, err
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(log.identity, opened) {
		_ = root.Close()
		return false, fmt.Errorf("Unity log directory changed: %s", log.directory)
	}
	info, err := root.Lstat(log.name)
	if errors.Is(err, fs.ErrNotExist) {
		_ = root.Close()
		_ = os.Remove(log.directory)
		return false, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		_ = root.Close()
		return false, fmt.Errorf("Unity log is not a regular file: %s", filepath.Join(log.directory, log.name))
	}
	succeeded, readErr := unityBuildSucceeded(root, log.name, info)
	_ = root.Close()
	if err := os.Rename(filepath.Join(log.directory, log.name), log.destination); err != nil {
		return succeeded, fmt.Errorf("log remains at %s: %w", filepath.Join(log.directory, log.name), err)
	}
	log.preserved = true
	_ = os.Remove(log.directory)
	return succeeded, readErr
}

func unityBuildSucceeded(root *os.Root, name string, info fs.FileInfo) (bool, error) {
	file, err := safefile.OpenCandidate(root, name)
	if err != nil {
		return false, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return false, fmt.Errorf("Unity log changed before reading")
	}
	start := max(int64(0), opened.Size()-unityLogTailLimit)
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return false, err
	}
	contents, err := io.ReadAll(io.LimitReader(file, unityLogTailLimit+1))
	if err != nil {
		return false, err
	}
	after, err := file.Stat()
	if err != nil || int64(len(contents)) != opened.Size()-start || after.Size() != opened.Size() || after.Mode() != opened.Mode() || !after.ModTime().Equal(opened.ModTime()) {
		return false, fmt.Errorf("Unity log changed while reading")
	}
	text := string(contents)
	if start > 0 {
		_, text, _ = strings.Cut(text, "\n") // Discard a possibly partial line.
	}
	var succeeded bool
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Build Finished, Result: ") {
			succeeded = line == "Build Finished, Result: Success."
		}
	}
	return succeeded, nil
}
