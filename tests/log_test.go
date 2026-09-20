package log

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func noTime(level Level) Options {
	return Options{MinimumLevel: level, IncludeTime: false}
}

func mustTextLogger(t *testing.T, writer io.Writer, options Options) *Logger {
	t.Helper()
	logger, err := text(writer, options)
	if err != nil {
		t.Fatalf("text() error = %v", err)
	}
	return logger
}

func mustJSONLogger(t *testing.T, writer io.Writer, options Options) *Logger {
	t.Helper()
	logger, err := json(writer, options)
	if err != nil {
		t.Fatalf("json() error = %v", err)
	}
	return logger
}

func TestDefaultOptions(t *testing.T) {
	got := defaultOptions()
	if got.MinimumLevel != LevelInfo || !got.IncludeTime {
		t.Fatalf("defaultOptions() = %#v", got)
	}
}

func TestTextFieldMatrix(t *testing.T) {
	group, err := Group("request", []Field{
		Text("method", "GET"),
		Integer("status", 200),
	})
	if err != nil {
		t.Fatal(err)
	}
	fields := []Field{
		Text("text", "hello world"),
		Integer("integer", -7),
		Integer64("integer64", 9223372036854775807),
		Number("number", 1.25),
		Boolean("boolean", true),
		Error("error", errors.New("broken")),
		Error("nil_error", nil),
		group,
	}

	var output bytes.Buffer
	logger := mustTextLogger(t, &output, noTime(LevelDebug))
	if err := logger.Info("hello \"Kinmokusei\"", fields); err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	want := "level=INFO message=\"hello \\\"Kinmokusei\\\"\"" +
		" text=\"hello world\" integer=-7 integer64=9223372036854775807" +
		" number=1.25 boolean=true error=\"broken\" nil_error=\"<nil>\"" +
		" request={method=\"GET\" status=200}\n"
	if got := output.String(); got != want {
		t.Fatalf("text output = %q, want %q", got, want)
	}
}

func TestJSONFieldMatrix(t *testing.T) {
	inner, err := Group("inner", []Field{Text("value", "nested")})
	if err != nil {
		t.Fatal(err)
	}
	group, err := Group("request", []Field{
		Text("method", "POST"),
		Integer("status", 201),
		inner,
	})
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	logger := mustJSONLogger(t, &output, noTime(LevelDebug))
	if err := logger.Warn("created", []Field{
		Text("text", "金木犀"),
		Integer("integer", -3),
		Integer64("integer64", 9007199254740993),
		Number("number", 2.5),
		Boolean("boolean", false),
		Error("error", errors.New("bad input")),
		group,
	}); err != nil {
		t.Fatalf("Warn() error = %v", err)
	}

	var decoded map[string]any
	if err := stdjson.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("JSON output is invalid: %v\n%s", err, output.String())
	}
	if decoded["level"] != "WARN" || decoded["message"] != "created" {
		t.Fatalf("decoded scalar values = %#v", decoded)
	}
	fields, ok := decoded["fields"].(map[string]any)
	if !ok || fields["text"] != "金木犀" || fields["integer"] != float64(-3) {
		t.Fatalf("decoded fields = %#v", decoded["fields"])
	}
	request, ok := fields["request"].(map[string]any)
	if !ok || request["method"] != "POST" || request["status"] != float64(201) {
		t.Fatalf("decoded request = %#v", decoded["request"])
	}
	nested, ok := request["inner"].(map[string]any)
	if !ok || nested["value"] != "nested" {
		t.Fatalf("decoded nested group = %#v", request["inner"])
	}
}

func TestLevelFilteringAndConvenienceMethods(t *testing.T) {
	var output bytes.Buffer
	logger := mustTextLogger(t, &output, noTime(LevelInfo))
	if logger.Enabled(LevelDebug) || !logger.Enabled(LevelInfo) || !logger.Enabled(LevelWarn) || !logger.Enabled(LevelError) {
		t.Fatalf("Enabled() returned unexpected matrix")
	}
	if err := logger.Debug("hidden", nil); err != nil {
		t.Fatal(err)
	}
	if err := logger.Info("info", nil); err != nil {
		t.Fatal(err)
	}
	if err := logger.Warn("warn", nil); err != nil {
		t.Fatal(err)
	}
	if err := logger.Error("error", nil); err != nil {
		t.Fatal(err)
	}
	if err := logger.Log(LevelDebug, "also hidden", nil); err != nil {
		t.Fatal(err)
	}
	want := "level=INFO message=\"info\"\n" +
		"level=WARN message=\"warn\"\n" +
		"level=ERROR message=\"error\"\n"
	if got := output.String(); got != want {
		t.Fatalf("filtered output = %q, want %q", got, want)
	}

	var debugOutput bytes.Buffer
	debugLogger := mustTextLogger(t, &debugOutput, noTime(LevelDebug))
	if err := debugLogger.Debug("visible debug", nil); err != nil {
		t.Fatal(err)
	}
	if got, want := debugOutput.String(), "level=DEBUG message=\"visible debug\"\n"; got != want {
		t.Fatalf("debug output = %q, want %q", got, want)
	}
}

func TestWithFieldsIsIndependentAndComposable(t *testing.T) {
	var output bytes.Buffer
	root := mustTextLogger(t, &output, noTime(LevelDebug))
	input := []Field{Text("component", "api")}
	child := root.WithFields(input)
	input[0] = Text("component", "mutated")
	grandchild := child.WithFields([]Field{Integer("request_id", 42)})

	if err := root.Info("root", nil); err != nil {
		t.Fatal(err)
	}
	if err := child.Info("child", nil); err != nil {
		t.Fatal(err)
	}
	if err := grandchild.Info("grandchild", []Field{Boolean("cached", true)}); err != nil {
		t.Fatal(err)
	}
	want := "level=INFO message=\"root\"\n" +
		"level=INFO message=\"child\" component=\"api\"\n" +
		"level=INFO message=\"grandchild\" component=\"api\" request_id=42 cached=true\n"
	if got := output.String(); got != want {
		t.Fatalf("composed output = %q, want %q", got, want)
	}
}

func TestTimeOption(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		var output bytes.Buffer
		logger := mustTextLogger(t, &output, Options{MinimumLevel: LevelInfo, IncludeTime: true})
		if err := logger.Info("timed", nil); err != nil {
			t.Fatal(err)
		}
		line := strings.TrimSpace(output.String())
		if !strings.HasPrefix(line, `time="`) || !strings.Contains(line, `" level=INFO message="timed"`) {
			t.Fatalf("timed text output = %q", line)
		}
		start := strings.Index(line, `time="`) + len(`time="`)
		end := strings.Index(line[start:], `" level=`)
		if end < 0 {
			t.Fatalf("time field not terminated: %q", line)
		}
		if _, err := time.Parse(time.RFC3339Nano, line[start:start+end]); err != nil {
			t.Fatalf("time value is invalid: %v", err)
		}
	})

	t.Run("json", func(t *testing.T) {
		var output bytes.Buffer
		logger := mustJSONLogger(t, &output, Options{MinimumLevel: LevelInfo, IncludeTime: true})
		if err := logger.Info("timed", nil); err != nil {
			t.Fatal(err)
		}
		var decoded struct {
			Time string `json:"time"`
		}
		if err := stdjson.Unmarshal(output.Bytes(), &decoded); err != nil {
			t.Fatal(err)
		}
		if _, err := time.Parse(time.RFC3339Nano, decoded.Time); err != nil {
			t.Fatalf("JSON time value is invalid: %v", err)
		}
	})
}

func TestConstructorErrorsAndStderr(t *testing.T) {
	if _, err := text(nil, noTime(LevelInfo)); err == nil || !strings.Contains(err.Error(), "must not be nil") {
		t.Fatalf("text(nil) error = %v", err)
	}
	if _, err := json(nil, noTime(LevelInfo)); err == nil || !strings.Contains(err.Error(), "must not be nil") {
		t.Fatalf("json(nil) error = %v", err)
	}
	if logger := stderr(noTime(LevelInfo)); logger == nil {
		t.Fatalf("stderr() = nil")
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

type shortWriter struct{}

func (shortWriter) Write(value []byte) (int, error) {
	return len(value) - 1, nil
}

func TestWriterFailures(t *testing.T) {
	if err := mustTextLogger(t, errorWriter{}, noTime(LevelInfo)).Info("message", nil); err == nil || err.Error() != "write failed" {
		t.Fatalf("error writer result = %v", err)
	}
	if err := mustTextLogger(t, shortWriter{}, noTime(LevelInfo)).Info("message", nil); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short writer result = %v", err)
	}
}

func TestJSONRejectsUnsupportedNumber(t *testing.T) {
	if _, err := Group("bad", []Field{Number("value", math.NaN())}); err == nil {
		t.Fatalf("Group(NaN) error = nil")
	}
	var output bytes.Buffer
	logger := mustJSONLogger(t, &output, noTime(LevelInfo))
	if err := logger.Info("bad", []Field{Number("value", math.Inf(1))}); err == nil {
		t.Fatalf("Info(+Inf) error = nil")
	}
	if output.Len() != 0 {
		t.Fatalf("invalid JSON value wrote partial output: %q", output.String())
	}
}

func TestConcurrentWritesRemainWholeLines(t *testing.T) {
	var output bytes.Buffer
	logger := mustJSONLogger(t, &output, noTime(LevelDebug)).WithFields([]Field{Text("component", "worker")})
	const workers = 16
	const messages = 50
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		worker := worker
		wait.Add(1)
		go func() {
			defer wait.Done()
			for message := 0; message < messages; message++ {
				if err := logger.Info("work", []Field{Integer("worker", worker), Integer("message", message)}); err != nil {
					t.Errorf("Info() error = %v", err)
					return
				}
			}
		}()
	}
	wait.Wait()

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if got, want := len(lines), workers*messages; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
	for index, line := range lines {
		var decoded map[string]any
		if err := stdjson.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("line %d is interleaved or invalid: %v: %q", index, err, line)
		}
		fields, ok := decoded["fields"].(map[string]any)
		if !ok || fields["component"] != "worker" || decoded["message"] != "work" {
			t.Fatalf("line %d values = %#v", index, decoded)
		}
	}
}

func TestTextNumberFormatting(t *testing.T) {
	values := []float64{0, -0.5, 1e20, math.Inf(1), math.Inf(-1), math.NaN()}
	for _, value := range values {
		var output bytes.Buffer
		logger := mustTextLogger(t, &output, noTime(LevelInfo))
		if err := logger.Info("number", []Field{Number("value", value)}); err != nil {
			t.Fatalf("Info(%v) error = %v", value, err)
		}
		if !strings.Contains(output.String(), "value="+strconv.FormatFloat(value, 'g', -1, 64)) {
			t.Fatalf("number output = %q for %v", output.String(), value)
		}
	}
}
