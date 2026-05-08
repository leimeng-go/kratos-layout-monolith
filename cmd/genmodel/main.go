package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	sqlFile := flag.String("sql", "", "Path to SQL DDL file")
	dir := flag.String("dir", "", "Directory containing SQL files")
	out := flag.String("out", "", "Output directory (default: current dir)")
	biz := flag.String("biz", "", "Biz package import path (e.g., github.com/.../moduser/biz)")
	module := flag.String("module", "", "Go module path (default: read from go.mod in cwd)")
	flag.Parse()

	if *sqlFile == "" && *dir == "" {
		fmt.Fprintln(os.Stderr, "Error: --sql or --dir is required")
		flag.Usage()
		os.Exit(1)
	}

	var sqlFiles []string
	if *sqlFile != "" {
		sqlFiles = append(sqlFiles, *sqlFile)
	}
	if *dir != "" {
		entries, err := os.ReadDir(*dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading directory: %v\n", err)
			os.Exit(1)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				sqlFiles = append(sqlFiles, filepath.Join(*dir, e.Name()))
			}
		}
	}

	outDir := *out
	if outDir == "" {
		outDir = "."
	}

	for _, f := range sqlFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", f, err)
			os.Exit(1)
		}

		stmts := splitStatements(string(content))
		for _, stmt := range stmts {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || !strings.Contains(strings.ToUpper(stmt), "CREATE TABLE") {
				continue
			}

			table, err := ParseSQL(stmt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", f, err)
				os.Exit(1)
			}

			bizImport := *biz
			if bizImport == "" {
				mod := *module
				if mod == "" {
					mod = guessModulePath()
				}
				absOut, _ := filepath.Abs(outDir)
				parentDir := filepath.Dir(absOut)
				if filepath.Base(absOut) == "data" {
					bizImport = mod + filepath.ToSlash(parentDir) + "/biz"
				}
			}

			pkgName := filepath.Base(outDir)
			modulePath := *module
		if modulePath == "" {
			modulePath = guessModulePath()
		}

			if err := Generate(table, pkgName, modulePath, bizImport, outDir); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating %s: %v\n", table.Name, err)
				os.Exit(1)
			}

			fmt.Printf("Generated: %s -> %s\n", table.Name, outDir)
		}
	}
}

func splitStatements(content string) []string {
	var stmts []string
	var current strings.Builder
	for _, line := range strings.Split(content, "\n") {
		current.WriteString(line)
		current.WriteString("\n")
		if strings.Contains(line, ");") {
			stmts = append(stmts, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		stmts = append(stmts, current.String())
	}
	return stmts
}

func guessModulePath() string {
	content, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module ")
		}
	}
	return ""
}
