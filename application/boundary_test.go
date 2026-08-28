package main_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestCurraImportBoundary(t *testing.T) {
	const forbiddenPrefix = "github.com/sPreetham42/timetable-platform/internal/scheduler"
	allowedPackagePrefix := filepath.Join("internal", "curra")

	fset := token.NewFileSet()
	var violations []string

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Normalize path
		cleanPath := filepath.Clean(path)
		if strings.Contains(cleanPath, allowedPackagePrefix) {
			return nil // Allowed adapter package
		}

		node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("failed to parse %s: %v", path, err)
		}

		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(importPath, forbiddenPrefix) {
				violations = append(violations, cleanPath+": imports "+importPath)
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("failed to scan application files: %v", err)
	}

	if len(violations) > 0 {
		t.Fatalf("CURRA import boundary violated! Only application/internal/curra may import CURRA internals.\nViolations:\n%s",
			strings.Join(violations, "\n"))
	}
}
