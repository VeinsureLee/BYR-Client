package buildflow

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// RunCommand is a function Strategy. Tests replace subprocess execution without
// installing a GUI framework or invoking a mobile toolchain.
type RunCommand func(context.Context, string, map[string]string, string, ...string) error

func Exec(stdout, stderr io.Writer) RunCommand {
	return func(ctx context.Context, directory string, environment map[string]string, name string, arguments ...string) error {
		command := exec.CommandContext(ctx, name, arguments...)
		command.Dir = directory
		command.Env = mergeEnvironment(os.Environ(), environment)
		command.Stdout, command.Stderr, command.Stdin = stdout, stderr, os.Stdin
		if err := command.Run(); err != nil {
			return fmt.Errorf("%s %s: %w", name, strings.Join(arguments, " "), err)
		}
		return nil
	}
}

func mergeEnvironment(original []string, overrides map[string]string) []string {
	keyOf := func(key string) string {
		if runtime.GOOS == "windows" {
			return strings.ToUpper(key)
		}
		return key
	}
	values := make(map[string]string)
	for _, entry := range original {
		key, value, found := strings.Cut(entry, "=")
		if found {
			values[keyOf(key)] = key + "=" + value
		}
	}
	for key, value := range overrides {
		values[keyOf(key)] = key + "=" + value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

func FindRoot(start string) (string, error) {
	directory, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if fileExists(filepath.Join(directory, "go.work")) && fileExists(filepath.Join(directory, "core", "go.mod")) && fileExists(filepath.Join(directory, "apps", "desktop", "go.mod")) {
			return filepath.EvalSymlinks(directory)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("not inside the BYR Go workspace")
		}
		directory = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
