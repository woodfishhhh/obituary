package cmdutil

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestHumanOutputUsesCatalog(t *testing.T) {
	fset := token.NewFileSet()
	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch entry.Name() {
		case "mcp", "hook", "clitest", "remove", "gitclean", "removeitem", "rm", "rmdir":
			if entry.IsDir() {
				return filepath.SkipDir
			}
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			s, ok := c.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch s.Sel.Name {
			case "Printf", "Println", "Print", "Fprintf", "Fprint", "Fprintln", "Errorf", "Sprintf":
			case "New":
				if pkg, ok := s.X.(*ast.Ident); !ok || pkg.Name != "errors" {
					return true
				}
			default:
				return true
			}
			if s.Sel.Name == "Fprintf" && len(c.Args) > 1 {
				if f, ok := c.Args[1].(*ast.BasicLit); ok && f.Value == `"  systemctl --user %s --now %s\n"` {
					return true
				}
			}
			for _, arg := range c.Args {
				inspectOutputLiteral(t, fset, arg)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

var formatVerb = regexp.MustCompile(`%[-+#0-9.]*[a-zA-Z%]`)
var displayWord = regexp.MustCompile(`[A-Za-z]+|\p{Han}`)

var literalData = map[string]bool{
	"%q hook %s":                     true, // The installed hook command is an agent contract.
	"one or more items failed":       true, // ErrReported is an internal sentinel suppressed by main.
	"  launchctl enable gui/%d/%s\n": true, "  launchctl bootstrap gui/%d %q\n": true,
	"  systemctl --user daemon-reload": true, "  systemctl --user %s --now %s\n": true,
	"  %-9s HKCU\\Software\\Classes\\%s\n": true, "  %-9s HKCU Run %s %s\n": true,
	"  Agent Skill → %s%s\n": true, "%s Agent Skill → %s\n": true,
}

// Syntax printed for copying is not prose. Match complete lines so additional
// English in the same output still fails the check.
var configSyntax = regexp.MustCompile(`^(?:\[[a-z_]+\]|[a-z_]+ = %[qtd])$`)

func syntaxOnly(text string) bool {
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if !configSyntax.MatchString(line) {
			return false
		}
	}
	return true
}

func inspectOutputLiteral(t *testing.T, fset *token.FileSet, expr ast.Expr) {
	switch x := expr.(type) {
	case *ast.BinaryExpr:
		inspectOutputLiteral(t, fset, x.X)
		inspectOutputLiteral(t, fset, x.Y)
	case *ast.Ident:
		if x.Obj == nil {
			return
		}
		if decl, ok := x.Obj.Decl.(*ast.AssignStmt); ok && len(decl.Rhs) == 1 {
			if lit, ok := decl.Rhs[0].(*ast.BasicLit); ok {
				inspectOutputLiteral(t, fset, lit)
			}
		}
	case *ast.CallExpr:
		if s, ok := x.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Sprintf" {
			for _, a := range x.Args {
				inspectOutputLiteral(t, fset, a)
			}
		}
	case *ast.BasicLit:
		if x.Kind != token.STRING {
			return
		}
		text, err := strconv.Unquote(x.Value)
		if err != nil {
			t.Fatal(err)
		}
		if displayWord.MatchString(formatVerb.ReplaceAllString(text, "")) && !syntaxOnly(text) && !literalData[text] {
			t.Errorf("%s: human output bypasses catalog: %q", fset.Position(x.Pos()), text)
		}
	}
}
