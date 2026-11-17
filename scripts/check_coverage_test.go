package main

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func TestMeasureSeparatesCompilerSupportFromLibraryStatements(t *testing.T) {
	source := []byte("package example\nfunc library(value *int) int {\n if value == nil {\n  panic(\"non-null parameter value failed at example.km:1:1\")\n }\n return *value\n}\nfunc helper() { println(0) }\n")
	start := strings.Index(string(source), "func library")
	end := strings.Index(string(source), "func helper")
	mapping := []byte(fmt.Sprintf(`{"version":1,"generated":{"file":"example.go","sha256":"%x","byte_length":%d},"mappings":[{"generated":{"start":{"offset":%d},"end":{"offset":%d}}}]}`, sha256.Sum256(source), len(source), start, end))
	profile := "mode: atomic\nexample.com/example/example.go:2.30,3.18 1 1\nexample.com/example/example.go:3.18,5.3 1 0\nexample.com/example/example.go:6.2,6.15 1 1\nexample.com/example/example.go:8.15,8.29 1 0\n"
	result, err := measure(source, mapping, []byte(profile))
	if err != nil || result != (coverage{covered: 2, total: 2, excluded: 2}) {
		t.Fatalf("measure = %+v, %v", result, err)
	}
	t.Run("uncovered library statements remain failures", func(t *testing.T) {
		result, err := measure(source, mapping, []byte(strings.Replace(profile, "6.2,6.15 1 1", "6.2,6.15 1 0", 1)))
		if err != nil || result.covered != 1 || result.total != 2 {
			t.Fatalf("measure = %+v, %v", result, err)
		}
	})
	for name, input := range map[string]string{
		"unknown file": strings.ReplaceAll(profile, "example.go:", "other.go:"),
		"bad position": strings.Replace(profile, "6.2,6.15", "600.2,600.15", 1),
		"bad column":   strings.Replace(profile, "6.2,6.15", "6.2,6.150", 1),
		"bad header":   strings.Replace(profile, "mode: atomic", "mode: invalid", 1),
		"bad block":    "mode: set\ninvalid\n",
		"empty":        "mode: set\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := measure(source, mapping, []byte(input)); err == nil {
				t.Fatal("malformed coverage profile accepted")
			}
		})
	}
	t.Run("stale map", func(t *testing.T) {
		if _, err := measure(append(source, '\n'), mapping, []byte(profile)); err == nil {
			t.Fatal("stale source map accepted")
		}
	})
}
