package userdirs

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
)

type directoryRead struct{ file, variable string }

var directoryExemptions = map[directoryRead]string{
	{"internal/testenv/desktop_windows.go", "windows.KnownFolderPath"}: "Read-only leak snapshots must inspect real shell folders despite test directory overrides",
	{"internal/testenv/fakeclaude.go", "CLAUDE_CONFIG_DIR"}:            "The test-only fake requires an explicit redirected directory; userdirs would create a TestMain import cycle",
	{"internal/core/safety.go", "USERPROFILE"}:                         "DefaultGuard protects real user locations; it does not store data there",
	{"internal/core/safety.go", "APPDATA"}:                             "DefaultGuard protects real user locations; it does not store data there",
	{"internal/core/safety.go", "LOCALAPPDATA"}:                        "DefaultGuard protects real user locations; it does not store data there",
	{"internal/app/guard_windows.go", "windows.KnownFolderPath"}:       "Guard and Explorer labels use redirected known folders; state stays in userdirs",
	{"internal/hook/discovery_windows.go", "windows.KnownFolderPath"}:  "Discovery diagnostics classify system locations without reading or storing user data there",
}

// directoryReads follows local function aliases too (DefaultGuard uses env :=
// os.Getenv). Comments, string contents and os.Setenv are not directory reads.
func directoryReads(file *ast.File) []string {
	imports := map[string]string{}
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = path
	}
	var resolve func(ast.Expr) (string, bool)
	resolve = func(expr ast.Expr) (string, bool) {
		switch e := expr.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := e.X.(*ast.Ident); ok {
				if path := imports[pkg.Name]; path == "os" || path == "golang.org/x/sys/windows" {
					return filepath.Base(path) + "." + e.Sel.Name, true
				}
			}
		case *ast.Ident:
			if e.Obj != nil {
				switch decl := e.Obj.Decl.(type) {
				case *ast.AssignStmt:
					for i, lhs := range decl.Lhs {
						if id, ok := lhs.(*ast.Ident); ok && id.Obj == e.Obj && i < len(decl.Rhs) {
							return resolve(decl.Rhs[i])
						}
					}
				case *ast.ValueSpec:
					for i, id := range decl.Names {
						if id.Obj == e.Obj && i < len(decl.Values) {
							return resolve(decl.Values[i])
						}
					}
				}
			}
		case *ast.BasicLit:
			if e.Kind == token.STRING {
				value, err := strconv.Unquote(e.Value)
				return value, err == nil
			}
		case *ast.ParenExpr:
			return resolve(e.X)
		case *ast.BinaryExpr:
			if e.Op == token.ADD {
				left, lok := resolve(e.X)
				right, rok := resolve(e.Y)
				if lok && rok {
					return left + right, true
				}
			}
		}
		return "", false
	}
	blocked := map[string]bool{}
	blocked["CODEBUDDY_CONFIG_DIR"] = true
	blocked["ZCODE_DATA_BASE_DIR"] = true
	blocked["QODER_CONFIG_DIR"], blocked["QODERCN_CONFIG_DIR"] = true, true
	for _, name := range strings.Fields("LOCALAPPDATA APPDATA USERPROFILE HOME XDG_DATA_HOME XDG_CONFIG_HOME CLAUDE_CONFIG_DIR CODEX_HOME VIBE_HOME KIMI_SHARE_DIR COPILOT_HOME GEMINI_CLI_HOME FACTORY_HOME_OVERRIDE HERMES_HOME HERMES_DATA_DIR_SUFFIX CLINE_DIR CLINE_DATA_DIR KILO_CONFIG_DIR OPENCODE_CONFIG_DIR OPENCODE_CONFIG OPENCLAW_STATE_DIR OPENCLAW_HOME OPENCLAW_CONFIG_PATH OPENCLAW_PROFILE PI_CODING_AGENT_DIR PI_CONFIG_DIR OBIT_HOME OBIT_CONFIG_DIR OBIT_USER_HOME DSH_HOME") {
		blocked[name] = true
	}
	for _, name := range []string{"MINIMAX_DATA_DIR", "MAVIS_DATA_DIR", "MCODE_INSTALL_DIR"} {
		blocked[name] = true
	}
	var found []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name, _ := resolve(call.Fun)
		switch name {
		case "os.UserHomeDir", "os.UserConfigDir", "os.UserCacheDir", "windows.KnownFolderPath":
			found = append(found, name)
		case "os.Getenv", "os.LookupEnv":
			if len(call.Args) == 1 {
				if variable, ok := resolve(call.Args[0]); ok && blocked[strings.ToUpper(variable)] {
					found = append(found, strings.ToUpper(variable))
				}
			}
		}
		return true
	})
	return found
}

func directoryViolations(reads []directoryRead, exemptions map[directoryRead]string) []string {
	used := map[directoryRead]bool{}
	var violations []string
	for _, read := range reads {
		if exemptions[read] == "" {
			violations = append(violations, fmt.Sprintf("%s reads %s outside userdirs", read.file, read.variable))
		} else {
			used[read] = true
		}
	}
	for exempt := range exemptions {
		if !used[exempt] {
			violations = append(violations, fmt.Sprintf("unused exemption: %s, %s", exempt.file, exempt.variable))
		}
	}
	return violations
}
