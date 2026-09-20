package log

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func TestLevelThresholdMatrix(t *testing.T) {
	levels := []struct {
		name  string
		level Level
	}{
		{"debug", LevelDebug},
		{"info", LevelInfo},
		{"warn", LevelWarn},
		{"error", LevelError},
	}
	for minimumIndex, minimum := range levels {
		for eventIndex, event := range levels {
			t.Run(minimum.name+"_minimum_"+event.name+"_event", func(t *testing.T) {
				var output bytes.Buffer
				logger := mustTextLogger(t, &output, noTime(minimum.level))
				if got, want := logger.Enabled(event.level), eventIndex >= minimumIndex; got != want {
					t.Fatalf("Enabled(%v) = %v, want %v", event.level, got, want)
				}
				if err := logger.Log(event.level, event.name, nil); err != nil {
					t.Fatalf("Log() error = %v", err)
				}
				if eventIndex < minimumIndex {
					if output.Len() != 0 {
						t.Fatalf("filtered event output = %q", output.String())
					}
				} else if !strings.Contains(output.String(), "message=\""+event.name+"\"") {
					t.Fatalf("enabled event output = %q", output.String())
				}
			})
		}
	}
}

func TestFieldNameValidationMatrix(t *testing.T) {
	valid := []string{"A", "_", "field1", "http.method", "request-id", "a_b.c-d"}
	for _, name := range valid {
		for _, format := range []string{"text", "json"} {
			t.Run("valid_"+format+"_"+name, func(t *testing.T) {
				var output bytes.Buffer
				var logger *Logger
				if format == "json" {
					logger = mustJSONLogger(t, &output, noTime(LevelInfo))
				} else {
					logger = mustTextLogger(t, &output, noTime(LevelInfo))
				}
				if err := logger.Info("valid", []Field{Text(name, "value")}); err != nil {
					t.Fatalf("Info(%q) error = %v", name, err)
				}
				if output.Len() == 0 {
					t.Fatalf("Info(%q) produced no output", name)
				}
			})
		}
	}

	invalid := []string{"", "1field", ".field", "-field", "bad name", "bad\tname", "bad\nname", "bad=name", "bad/name", "bad{name", "名前"}
	for _, name := range invalid {
		for _, format := range []string{"text", "json"} {
			t.Run("invalid_"+format, func(t *testing.T) {
				var output bytes.Buffer
				var logger *Logger
				if format == "json" {
					logger = mustJSONLogger(t, &output, noTime(LevelInfo))
				} else {
					logger = mustTextLogger(t, &output, noTime(LevelInfo))
				}
				if err := logger.Info("invalid", []Field{Text(name, "value")}); err == nil {
					t.Fatalf("Info(%q) error = nil", name)
				}
				if output.Len() != 0 {
					t.Fatalf("Info(%q) wrote %q", name, output.String())
				}
			})
		}
	}
}

func TestDuplicateFieldMatrix(t *testing.T) {
	tests := []struct {
		name   string
		base   []Field
		record []Field
	}{
		{"record duplicate", nil, []Field{Text("same", "a"), Text("same", "b")}},
		{"base duplicate", []Field{Text("same", "a"), Text("same", "b")}, nil},
		{"base record collision", []Field{Text("same", "a")}, []Field{Text("same", "b")}},
	}
	for _, test := range tests {
		for _, format := range []string{"text", "json"} {
			t.Run(test.name+"_"+format, func(t *testing.T) {
				var output bytes.Buffer
				var root *Logger
				if format == "json" {
					root = mustJSONLogger(t, &output, noTime(LevelInfo))
				} else {
					root = mustTextLogger(t, &output, noTime(LevelInfo))
				}
				logger, contextErr := root.WithFields(test.base)
				if contextErr != nil {
					if test.name != "base duplicate" || !strings.Contains(contextErr.Error(), "duplicate log field name") {
						t.Fatalf("WithFields() error = %v", contextErr)
					}
					if output.Len() != 0 {
						t.Fatalf("invalid persistent fields wrote %q", output.String())
					}
					return
				}
				err := logger.Info("duplicate", test.record)
				if err == nil || !strings.Contains(err.Error(), "duplicate log field name") {
					t.Fatalf("Info() error = %v", err)
				}
				if output.Len() != 0 {
					t.Fatalf("duplicate fields wrote %q", output.String())
				}
			})
		}
	}
}

func TestWithFieldsRejectsInvalidContextImmediately(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			var root *Logger
			if format == "json" {
				root = mustJSONLogger(t, &output, noTime(LevelInfo))
			} else {
				root = mustTextLogger(t, &output, noTime(LevelInfo))
			}
			if child, err := root.WithFields([]Field{Text("bad name", "value")}); err == nil || child != nil {
				t.Fatalf("WithFields(invalid) = (%v, %v)", child, err)
			}
			if output.Len() != 0 {
				t.Fatalf("invalid context wrote %q", output.String())
			}
		})
	}
}

func TestGroupValidationAndSnapshot(t *testing.T) {
	invalidNames := []string{"", "1group", "bad group", "グループ"}
	for _, name := range invalidNames {
		if _, err := Group(name, nil); err == nil {
			t.Errorf("Group(%q) error = nil", name)
		}
	}
	if _, err := Group("group", []Field{Text("same", "a"), Text("same", "b")}); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("Group(duplicate) error = %v", err)
	}
	if _, err := Group("group", []Field{Text("bad name", "value")}); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("Group(invalid child) error = %v", err)
	}

	empty, err := Group("empty", nil)
	if err != nil {
		t.Fatalf("Group(empty) error = %v", err)
	}
	children := []Field{Text("value", "original")}
	snapshot, err := Group("snapshot", children)
	if err != nil {
		t.Fatal(err)
	}
	children[0] = Text("value", "mutated")

	var output bytes.Buffer
	logger := mustJSONLogger(t, &output, noTime(LevelInfo))
	if err := logger.Info("groups", []Field{empty, snapshot}); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := stdjson.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	fields := decoded["fields"].(map[string]any)
	if got := fields["empty"].(map[string]any); len(got) != 0 {
		t.Fatalf("empty group = %#v", got)
	}
	if got := fields["snapshot"].(map[string]any)["value"]; got != "original" {
		t.Fatalf("snapshot value = %#v", got)
	}
}

type trackingWriter struct {
	calls int
	delta int
	err   error
}

func (writer *trackingWriter) Write(value []byte) (int, error) {
	writer.calls++
	return len(value) + writer.delta, writer.err
}

func TestWriterContractMatrix(t *testing.T) {
	tests := []struct {
		name    string
		delta   int
		err     error
		wantErr string
	}{
		{"exact write", 0, nil, ""},
		{"short by one", -1, nil, io.ErrShortWrite.Error()},
		{"negative count", -1000, nil, io.ErrShortWrite.Error()},
		{"oversized count", 1, nil, io.ErrShortWrite.Error()},
		{"error with zero", -1000, errors.New("writer failed"), "writer failed"},
		{"error after full count", 0, errors.New("late failure"), "late failure"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writer := &trackingWriter{delta: test.delta, err: test.err}
			logger := mustTextLogger(t, writer, noTime(LevelInfo))
			err := logger.Info("write", nil)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("Info() error = %v", err)
				}
			} else if err == nil || err.Error() != test.wantErr {
				t.Fatalf("Info() error = %v, want %q", err, test.wantErr)
			}
			if writer.calls != 1 {
				t.Fatalf("Write() calls = %d, want 1", writer.calls)
			}
		})
	}
}

func TestFilteredRecordHasNoSideEffects(t *testing.T) {
	writer := &trackingWriter{err: errors.New("must not be called")}
	logger := mustJSONLogger(t, writer, noTime(LevelError))
	err := logger.Info("filtered", []Field{
		Text("bad name", "invalid but filtered"),
		Text("duplicate", "a"),
		Text("duplicate", "b"),
		Number("infinite", math.Inf(1)),
	})
	if err != nil {
		t.Fatalf("filtered Info() error = %v", err)
	}
	if writer.calls != 0 {
		t.Fatalf("filtered Info() write calls = %d", writer.calls)
	}
}

func TestMetadataCannotBeOverwritten(t *testing.T) {
	var output bytes.Buffer
	logger := mustJSONLogger(t, &output, noTime(LevelInfo))
	if err := logger.Info("metadata", []Field{
		Text("message", "user message"),
		Text("level", "user level"),
		Text("time", "user time"),
	}); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := stdjson.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["message"] != "metadata" || decoded["level"] != "INFO" {
		t.Fatalf("metadata = %#v", decoded)
	}
	fields := decoded["fields"].(map[string]any)
	if fields["message"] != "user message" || fields["level"] != "user level" || fields["time"] != "user time" {
		t.Fatalf("user fields = %#v", fields)
	}
}

func TestEscapingMatrix(t *testing.T) {
	values := []string{
		"",
		"plain",
		"quote \" slash \\",
		"line one\nline two\r\ttab",
		"control\x00byte",
		"金木犀🌳",
		string([]byte{0xff, 0xfe, 'x'}),
	}
	for index, value := range values {
		t.Run("json", func(t *testing.T) {
			var output bytes.Buffer
			logger := mustJSONLogger(t, &output, noTime(LevelInfo))
			if err := logger.Info(value, []Field{Text("value", value)}); err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := stdjson.Unmarshal(output.Bytes(), &decoded); err != nil {
				t.Fatalf("case %d invalid JSON: %v", index, err)
			}
			if utf8.ValidString(value) && decoded["message"] != value {
				t.Fatalf("case %d message = %#v, want %q", index, decoded["message"], value)
			}
		})
		t.Run("text", func(t *testing.T) {
			var output bytes.Buffer
			logger := mustTextLogger(t, &output, noTime(LevelInfo))
			if err := logger.Info(value, []Field{Text("value", value)}); err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(output.String(), "\n"); got != 1 {
				t.Fatalf("case %d physical lines = %d, output %q", index, got, output.String())
			}
		})
	}
}

func TestConcurrentTextWritesRemainWholeLines(t *testing.T) {
	var output bytes.Buffer
	root := mustTextLogger(t, &output, noTime(LevelDebug))
	children := []*Logger{
		mustWithFields(t, root, []Field{Text("worker", "a")}),
		mustWithFields(t, root, []Field{Text("worker", "b")}),
		mustWithFields(t, root, []Field{Text("worker", "c")}),
		mustWithFields(t, root, []Field{Text("worker", "d")}),
	}
	const records = 100
	var wait sync.WaitGroup
	for _, logger := range children {
		logger := logger
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := 0; index < records; index++ {
				if err := logger.Info("concurrent", []Field{Integer("index", index)}); err != nil {
					t.Errorf("Info() error = %v", err)
					return
				}
			}
		}()
	}
	wait.Wait()
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if got, want := len(lines), len(children)*records; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
	for index, line := range lines {
		if !strings.HasPrefix(line, `level=INFO message="concurrent" worker="`) || !strings.Contains(line, ` index=`) {
			t.Fatalf("line %d is interleaved: %q", index, line)
		}
	}
}
