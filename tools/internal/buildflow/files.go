package buildflow

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// requireInside resolves existing links before any recursive cleanup or move.
func requireInside(parent, path string) error {
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return err
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if os.IsNotExist(err) {
		resolvedPath, err = filepath.Abs(path)
	}
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(resolvedParent, resolvedPath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("generated output must remain inside %s: %s", parent, path)
	}
	return nil
}

func removeGenerated(parent, path string) error {
	if err := requireInside(parent, path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func outputDirectory(project, name string) (string, error) {
	path := filepath.Join(project, name)
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}
	if err := requireInside(project, path); err != nil {
		return "", err
	}
	return path, nil
}

func zipDirectory(source, destination, executable string) (err error) {
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	archive := zip.NewWriter(file)
	defer func() {
		if closeErr := archive.Close(); err == nil {
			err = closeErr
		}
	}()
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == source {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		// Cross-compiling on Windows cannot preserve Unix execute bits in
		// NTFS metadata. Record the target entry point and shell scripts as
		// executable in the archive, rather than shipping an unusable bundle.
		if entry.IsDir() {
			header.SetMode(os.ModeDir | 0755)
		} else if info.Mode().IsRegular() && (relative == executable || strings.HasSuffix(relative, ".sh")) {
			header.SetMode(0755)
		}
		if entry.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_, err = io.WriteString(writer, target)
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unexpected build output: %s", path)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
