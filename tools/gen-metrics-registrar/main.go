// Copyright 2021 the Exposure Notifications Server authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This package is used to create a meta metrics import package.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

var (
	flagPkgName     = flag.String("pkg", "main", "name of the package to use for generated file")
	flagDestination = flag.String("dest", "-", "path to write generated code (default: stdout)")
	flagModuleRoot  = flag.String("module-root", "", "path to root of go module")
	flagModuleName  = flag.String("module-name", "", "name of the module (e.g. github.com/foo/bar)")

	tmpl = template.Must(template.New("metricsImports").Parse(`
// Code generated by gen-metrics-registrar. DO NOT EDIT.
package {{.PkgName}}

import (
{{- range .Imports }}
	_ "{{ . }}"
{{- end }}
)
`))
)

type TemplateData struct {
	PkgName string
	Imports []string
}

func main() {
	if err := realMain(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(2)
	}
}

func realMain() error {
	flag.Parse()

	if *flagModuleRoot == "" {
		return fmt.Errorf("missing -module-root")
	}

	root, err := filepath.Abs(*flagModuleRoot)
	if err != nil {
		return fmt.Errorf("failed to make absolute path: %w", err)
	}

	if *flagModuleName == "" {
		return fmt.Errorf("missing -module-name")
	}

	candidates := make(map[string]struct{}, 8)

	if err := filepath.WalkDir(root, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		pkgs, err := parser.ParseDir(token.NewFileSet(), pth, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		for _, pkg := range pkgs {
			for fileName, file := range pkg.Files {
				// Ignore tests.
				if strings.HasSuffix(fileName, "_test.go") {
					continue
				}

				// Quick check to see if the o11y package is imported.
				if !importsStats(file) {
					continue
				}

				// Slightly less performant check to see if there's an init function
				// and if the init function calls CollectViews.
				if !hasInitViewsFunc(file) {
					continue
				}

				// If we got this far, the file contains metrics view collection and
				// we should add it to the list.
				pth, err := filepath.Rel(root, fileName)
				if err != nil {
					return fmt.Errorf("failed to get relative path: %w", err)
				}
				pth = path.Dir(pth)
				candidates[pth] = struct{}{}

				// We found one for this package, so no need to look at more files.
				break
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to walk files: %w", err)
	}

	imports := make([]string, 0, len(candidates))
	for c := range candidates {
		imports = append(imports, path.Join(*flagModuleName, c))
	}

	var b bytes.Buffer
	if err := tmpl.Execute(&b, &TemplateData{
		PkgName: *flagPkgName,
		Imports: imports,
	}); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	formatted, err := format.Source(b.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format template: %w", err)
	}

	var out *os.File
	if *flagDestination == "-" {
		out = os.Stdout
	} else {
		fout, err := os.OpenFile(*flagDestination, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			panic(err)
		}
		defer fout.Close()
		out = fout
	}

	if _, err := out.Write(formatted); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	_ = out.Sync()
	_ = out.Close()
	return nil
}

// importsStats returns true if the node has imports for the opencensus stats
// package.
func importsStats(node *ast.File) bool {
	for _, i := range node.Imports {
		if i.Path.Value == `"go.opencensus.io/stats/view"` {
			return true
		}
	}
	return false
}

// hasInitViewsFunc returns true if the given file has an init func, false otherwise.
func hasInitViewsFunc(node *ast.File) bool {
	for _, d := range node.Decls {
		if typ, ok := d.(*ast.FuncDecl); ok && typ.Name.Name == "init" {
			str := printNode(typ)
			if strings.Contains(str, "CollectViews") {
				return true
			}
		}
	}
	return false
}

// printNode prints the ast.Node.
func printNode(n ast.Node) string {
	var b bytes.Buffer
	fset := token.NewFileSet()
	if err := printer.Fprint(&b, fset, n); err != nil {
		panic(err)
	}
	return b.String()
}