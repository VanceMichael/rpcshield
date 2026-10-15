package bazel

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/uber-research/rpcshield/go/common/utils"
)

// Fix expects import package error.
// Issue: The package is expected to be imported using the path in comment field.
// But downstream package is using explicit import path.
// Example: zap/zapcore/encoder.go --> zap/buffer
// Solution: explicitly remove the import comment.
// Origin Src: package buffer // import "go.uber.org/zap/buffer"
// Expect: package buffer
func (br *gopathWrapper) fixImportComment(ctx context.Context, runner utils.Runner, gopathRoot string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fix import comment canceled: %w", err)
	}
	_, patherr := os.Stat(gopathRoot)
	if os.IsNotExist(patherr) {
		return nil
	}
	// By default, the folder is read-only, relax it in order to rename it.
	if _, _, err := utils.ExecCommandContext(ctx, runner, "chmod", "-R", "777", gopathRoot); err != nil {
		return err
	}

	// Walk through all .go files in the gopath
	err := filepath.WalkDir(gopathRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// go_path's generated tree can contain a directory whose name happens to end in
		// ".go" (e.g. when a genrule/embedsrcs output collides with a package directory
		// name), so a suffix check alone isn't enough to tell us this is a readable Go
		// source file; os.ReadFile on a directory fails with "is a directory" and aborts
		// the whole walk. Skip directories explicitly.
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Read the original file content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %v", path, err)
		}

		// Parse the file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("failed to parse file %s: %v", path, err)
		}

		packageLine := fset.Position(node.Package).Line
		modified := false

		for _, cg := range node.Comments {
			filtered := []*ast.Comment{}
			for _, c := range cg.List {
				pos := fset.Position(c.Slash)
				// Same line as `package` and matches `//import` or `// import`
				if pos.Line == packageLine && strings.HasPrefix(c.Text, "//import") || strings.HasPrefix(c.Text, "// import") {
					lines := strings.Split(string(content), "\n")
					br.Logger.Debugw("Has import comment pattern", "file", path)
					br.Logger.Debugw("target line", "line", pos.Line)
					br.Logger.Debugw("target comment", "comment", c.Text)
					br.Logger.Debugw("target line", "line", lines[pos.Line-1])
					modified = true
					continue
				}
				filtered = append(filtered, c)
			}
			if len(filtered) == 0 {
				cg.List = nil
			} else {
				cg.List = filtered
			}
		}

		if !modified {
			return nil // skip rewriting if nothing changed
		}

		outFile, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %v", path, err)
		}
		defer outFile.Close()

		err = printer.Fprint(outFile, fset, node)
		if err != nil {
			return fmt.Errorf("failed to write AST to file %s: %v", path, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %v", err)
	}

	return nil
}
