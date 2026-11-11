package testenv_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/runkey"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestEveryPackageStartsWithFakeRunRegistry(t *testing.T) {
	if _, ok := runkey.Current.(*testenv.RunRegistry); !ok {
		t.Fatalf("native Run key exposed to package tests: %T", runkey.Current)
	}
	r := testenv.NewRunRegistry()
	if err := r.Set("owned", "command"); err != nil {
		t.Fatal(err)
	}
	if got, err := r.Get("owned"); err != nil || got != "command" {
		t.Fatalf("get %q %v", got, err)
	}
	if err := r.Delete("owned"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Get("owned"); got != "" {
		t.Fatal("delete did not take effect")
	}
}

// The real Run path stays at one boundary. Tests may only open/delete a
// key whose declaration proves the disposable prefix, never the real Run key.
func TestRegistrySourceBoundary(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, data, 0)
			if err != nil {
				return err
			}
			if filepath.ToSlash(path) == "../../internal/runkey/runkey_windows.go" || filepath.ToSlash(path) == "../../internal/testenv/desktop_windows.go" {
				// The leak detector reads the real Run key to catch bypassed isolation.
				return nil
			}
			ast.Inspect(file, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}
				value, _ := strconv.Unquote(literal.Value)
				if strings.HasSuffix(strings.ToLower(value), `\currentversion\`+"run") {
					t.Errorf("%s: real Run path outside native startup boundary", path)
				}
				return true
			})
			if strings.HasSuffix(path, "_test.go") {
				checkRegistryCalls(t, path, file)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func checkRegistryCalls(t *testing.T, path string, file *ast.File) {
	t.Helper()
	aliases := map[string]bool{}
	for _, imp := range file.Imports {
		name, _ := strconv.Unquote(imp.Path.Value)
		if name != "golang.org/x/sys/windows/registry" {
			continue
		}
		alias := "registry"
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		if alias == "." {
			t.Errorf("%s: dot registry import bypasses boundary", path)
		}
		aliases[alias] = true
	}
	keys := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		id, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		expr, ok := assign.Rhs[0].(*ast.BinaryExpr)
		if !ok || expr.Op != token.ADD {
			return true
		}
		prefix, ok := expr.X.(*ast.BasicLit)
		if ok && prefix.Value == "`Software\\obituary-test-`" {
			keys[id.Name] = true
		}
		return true
	})
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || !aliases[id.Name] {
			return true
		}
		if sel.Sel.Name != "OpenKey" && sel.Sel.Name != "CreateKey" && sel.Sel.Name != "DeleteKey" {
			return true
		}
		if len(call.Args) < 2 {
			t.Errorf("%s: invalid registry call", path)
			return true
		}
		key, ok := call.Args[1].(*ast.Ident)
		if !ok || !keys[key.Name] {
			t.Errorf("%s: native registry call outside boundary without disposable test key", path)
		}
		return true
	})
}
