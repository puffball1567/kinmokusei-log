package log

import (
	"bytes"
	stdjson "encoding/json"
	"testing"
)

func FuzzJSONLoggingNeverPanicsAndStaysValid(f *testing.F) {
	f.Add("message", "name", "value")
	f.Add("金木犀\n\"", "strange\"key", "line one\nline two")
	f.Add("", "", "")
	f.Fuzz(func(t *testing.T, message string, name string, value string) {
		var output bytes.Buffer
		logger, err := json(&output, Options{MinimumLevel: LevelDebug, IncludeTime: false})
		if err != nil {
			t.Fatal(err)
		}
		if err := logger.Info(message, []Field{Text(name, value)}); err != nil {
			if output.Len() != 0 {
				t.Fatalf("failed record produced output: %q", output.String())
			}
			return
		}
		var decoded map[string]any
		if err := stdjson.Unmarshal(output.Bytes(), &decoded); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
	})
}

func FuzzTextLoggingNeverPanics(f *testing.F) {
	f.Add("message", "name", "value")
	f.Add("金木犀\n\"", "key", "line one\nline two")
	f.Add("", "", "")
	f.Fuzz(func(t *testing.T, message string, name string, value string) {
		var output bytes.Buffer
		logger, err := text(&output, Options{MinimumLevel: LevelDebug, IncludeTime: false})
		if err != nil {
			t.Fatal(err)
		}
		if err := logger.Info(message, []Field{Text(name, value)}); err != nil {
			if output.Len() != 0 {
				t.Fatalf("failed record produced output: %q", output.String())
			}
			return
		}
	})
}

func FuzzGroupValidationNeverPanics(f *testing.F) {
	f.Add("request", "method", "GET")
	f.Add("", "", "")
	f.Add("group", "bad name", "value")
	f.Fuzz(func(t *testing.T, groupName string, childName string, value string) {
		group, groupError := Group(groupName, []Field{Text(childName, value)})
		if groupError != nil {
			return
		}
		var output bytes.Buffer
		logger, loggerError := json(&output, Options{MinimumLevel: LevelDebug, IncludeTime: false})
		if loggerError != nil {
			t.Fatal(loggerError)
		}
		if err := logger.Info("group", []Field{group}); err != nil {
			t.Fatalf("Info() error = %v", err)
		}
		var decoded map[string]any
		if err := stdjson.Unmarshal(output.Bytes(), &decoded); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
	})
}
