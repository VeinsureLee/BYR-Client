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

func TestForumValuesDoNotImportProtocolTransport(t *testing.T) {
	forbidden := []string{
		"net/http", "net/url", "mime/multipart", "crypto/aes", "crypto/ecdh", "crypto/ed25519",
		"github.com/VeinsureLee/BYR-Client/core/internal/byrhttp",
	}
	err := filepath.WalkDir("forum", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			name, _ := strconv.Unquote(spec.Path.Value)
			for _, forbiddenImport := range forbidden {
				if name == forbiddenImport {
					t.Errorf("shared forum values import protocol transport: %s -> %s", path, name)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPrivateProtocolAdapterIsImportedOnlyInsideCore(t *testing.T) {
	root := ".."
	adapter := "github.com/VeinsureLee/BYR-Client/core/internal/byrhttp"
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
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
			if name != adapter {
				continue
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if !strings.HasPrefix(filepath.ToSlash(relative), "core/") {
				t.Errorf("private protocol adapter imported outside core: %s -> %s", relative, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
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
