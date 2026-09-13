// Command lint applies the minimal pinned rule set (see DEVELOPMENT.md).
// It is not gofmt, not go vet, and not a static analyzer: it enforces the
// project's own machine-facing conventions that those tools do not cover.
// Pinned rule set v1:
//
//	L001  no fmt.Print/Printf/Println or builtin print/println outside
//	      cmd/, tools/, and _test.go files; printing belongs to CLI
//	      composition so library stdout stays machine-readable.
//	L002  no empty interface{} literals; use any.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type finding struct {
	Rule    string
	Path    string
	Message string
	Line    int
	Column  int
}

// analyze reports rule violations for one Go source file. Files that do not
// parse are skipped: the build step owns syntax errors.
func analyze(path string, src []byte) []finding {
	exemptFromL001 := strings.HasPrefix(filepath.ToSlash(path), "cmd/") ||
		strings.HasPrefix(filepath.ToSlash(path), "tools/") ||
		strings.HasSuffix(path, "_test.go")

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil
	}

	var findings []finding
	report := func(rule string, pos token.Pos, message string) {
		p := fset.Position(pos)
		findings = append(findings, finding{
			Rule: rule, Path: path, Message: message,
			Line: p.Line, Column: p.Column,
		})
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if exemptFromL001 {
				return true
			}
			if message, ok := strayPrint(node); ok {
				report("L001", node.Pos(), message)
			}
		case *ast.InterfaceType:
			if len(node.Methods.List) == 0 {
				report("L002", node.Pos(), "use any instead of empty interface{}")
			}
		}
		return true
	})
	return findings
}

func strayPrint(call *ast.CallExpr) (string, bool) {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "fmt" {
			switch fun.Sel.Name {
			case "Print", "Printf", "Println":
				return "fmt." + fun.Sel.Name + " prints outside CLI composition (L001)", true
			}
		}
	case *ast.Ident:
		if fun.Name == "print" || fun.Name == "println" {
			return "builtin " + fun.Name + " prints outside CLI composition (L001)", true
		}
	}
	return "", false
}

// run walks the tree, analyzes each Go file, and reports findings in
// deterministic walk order.
func run(root string, stdout io.Writer) int {
	var findings []finding
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".tmp", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		findings = append(findings, analyze(path, src)...)
		return nil
	})
	if err != nil {
		fmt.Fprintf(stdout, "lint: %v\n", err)
		return 1
	}
	for _, f := range findings {
		fmt.Fprintf(stdout, "%s:%d:%d: %s %s\n", f.Path, f.Line, f.Column, f.Rule, f.Message)
	}
	if len(findings) > 0 {
		return 1
	}
	return 0
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	os.Exit(run(root, os.Stdout))
}
