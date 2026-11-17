// Command check_coverage requires full coverage of source-mapped library code.
// Compiler runtime helpers and injected non-null panic paths are not library code.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type interval struct{ start, end int }

type sourceMap struct {
	Version   int `json:"version"`
	Generated struct {
		File       string `json:"file"`
		SHA256     string `json:"sha256"`
		ByteLength int    `json:"byte_length"`
	} `json:"generated"`
	Mappings []struct {
		Generated struct {
			Start struct{ Offset int } `json:"start"`
			End   struct{ Offset int } `json:"end"`
		} `json:"generated"`
	} `json:"mappings"`
}

type coverage struct{ covered, total, excluded int }

var profileBlock = regexp.MustCompile(`^(.+):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)$`)

func measure(source, mapping, profile []byte) (coverage, error) {
	var result coverage
	var metadata sourceMap
	if err := json.Unmarshal(mapping, &metadata); err != nil {
		return result, err
	}
	if metadata.Version != 1 || metadata.Generated.ByteLength != len(source) ||
		metadata.Generated.SHA256 != fmt.Sprintf("%x", sha256.Sum256(source)) || len(metadata.Mappings) == 0 {
		return result, fmt.Errorf("missing, unsupported or stale source map")
	}
	var mapped []interval
	for _, entry := range metadata.Mappings {
		span := interval{entry.Generated.Start.Offset, entry.Generated.End.Offset}
		if span.start < 0 || span.end <= span.start || span.end > len(source) {
			return result, fmt.Errorf("invalid source-map range")
		}
		mapped = append(mapped, span)
	}
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, metadata.Generated.File, source, 0)
	if err != nil {
		return result, err
	}
	var checks []interval
	ast.Inspect(file, func(node ast.Node) bool {
		guard, ok := node.(*ast.IfStmt)
		if !ok || len(guard.Body.List) != 1 {
			return true
		}
		statement, ok := guard.Body.List[0].(*ast.ExprStmt)
		if !ok {
			return true
		}
		call, ok := statement.X.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		callee, ok := call.Fun.(*ast.Ident)
		if !ok || callee.Name != "panic" {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		message, _ := strconv.Unquote(literal.Value)
		if strings.HasPrefix(message, "non-null ") && strings.Contains(message, " failed at ") {
			checks = append(checks, interval{positions.Position(guard.Body.Pos()).Offset, positions.Position(guard.Body.End()).Offset})
		}
		return true
	})
	lines := []int{0}
	for index, value := range source {
		if value == '\n' {
			lines = append(lines, index+1)
		}
	}
	offset := func(line, column int) (int, error) {
		if line < 1 || line > len(lines) || column < 1 {
			return 0, fmt.Errorf("invalid coverage position")
		}
		limit := len(source)
		if line < len(lines) {
			limit = lines[line] - 1
		}
		value := lines[line-1] + column - 1
		if value > limit {
			return 0, fmt.Errorf("coverage position exceeds line length")
		}
		return value, nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(profile))
	if !scanner.Scan() || (scanner.Text() != "mode: set" && scanner.Text() != "mode: count" && scanner.Text() != "mode: atomic") {
		return result, fmt.Errorf("invalid coverage header")
	}
	for scanner.Scan() {
		match := profileBlock.FindStringSubmatch(scanner.Text())
		if match == nil || filepath.Base(match[1]) != filepath.Base(metadata.Generated.File) {
			return result, fmt.Errorf("unexpected coverage block: %s", scanner.Text())
		}
		var values [6]int
		for index := range values {
			values[index], err = strconv.Atoi(match[index+2])
			if err != nil {
				return result, err
			}
		}
		start, err := offset(values[0], values[1])
		if err != nil {
			return result, err
		}
		end, err := offset(values[2], values[3])
		if err != nil || end < start {
			return result, fmt.Errorf("invalid coverage range")
		}
		included := false
		for _, span := range mapped {
			included = included || (start < span.end && end > span.start)
		}
		for _, span := range checks {
			if start >= span.start && end <= span.end {
				included = false
			}
		}
		if !included {
			result.excluded += values[4]
			continue
		}
		result.total += values[4]
		if values[5] > 0 {
			result.covered += values[4]
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	if result.total == 0 {
		return result, fmt.Errorf("no source-mapped library statements in coverage profile")
	}
	return result, nil
}

func run(args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: check_coverage generated.go source-map.json coverage.out")
	}
	var inputs [3][]byte
	for index, name := range args {
		var err error
		inputs[index], err = os.ReadFile(name)
		if err != nil {
			return err
		}
	}
	result, err := measure(inputs[0], inputs[1], inputs[2])
	if err != nil {
		return err
	}
	fmt.Printf("library statement coverage: %.1f%% (%d/%d; %d compiler support statements excluded)\n",
		100*float64(result.covered)/float64(result.total), result.covered, result.total, result.excluded)
	if result.covered != result.total {
		return fmt.Errorf("library statement coverage must be 100.0%%")
	}
	return nil
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if err := run(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
