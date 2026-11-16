package core_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCoreNeverImportsUIOrApplicationModules(t *testing.T) {
	forbidden := []string{"github.com/egoist/mygo", "fyne.io/", "gioui.org/", "github.com/VeinsureLee/BYR-Client/apps/"}
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			name, _ := strconv.Unquote(spec.Path.Value)
			for _, prefix := range forbidden {
				if strings.HasPrefix(name, prefix) {
					t.Errorf("shared core imports UI/platform code: %s -> %s", path, name)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
