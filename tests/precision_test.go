package log

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type typedNilWriter struct{}

func (*typedNilWriter) Write(value []byte) (int, error) {
	return len(value), nil
}

type typedNilError struct{}

func (*typedNilError) Error() string {
	return "must not be called"
}

type valueError string

func (value valueError) Error() string {
	return string(value)
}

func TestTypedNilInterfaceMatrix(t *testing.T) {
	var writer *typedNilWriter
	for _, test := range []struct {
		name      string
		construct func(io.Writer, Options) (*Logger, error)
	}{
		{"text", text},
		{"json", json},
	} {
		t.Run(test.name, func(t *testing.T) {
			logger, err := test.construct(writer, noTime(LevelInfo))
			if err == nil || logger != nil || !strings.Contains(err.Error(), "must not be nil") {
				t.Fatalf("constructor(typed nil) = (%v, %v)", logger, err)
			}
		})
	}

	var pointerError *typedNilError
	var output bytes.Buffer
	logger := mustTextLogger(t, &output, noTime(LevelInfo))
	if err := logger.Info("errors", []Field{
		Error("typed_nil", pointerError),
		Error("value_error", valueError("value failure")),
	}); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "level=INFO message=\"errors\" typed_nil=\"<nil>\" value_error=\"value failure\"\n"; got != want {
		t.Fatalf("typed error output = %q, want %q", got, want)
	}
}

func TestConvenienceFieldMatrix(t *testing.T) {
	timestamp := time.Date(2026, time.September, 20, 13, 14, 15, 123456789, time.FixedZone("JST", 9*60*60))
	fields := []Field{
		Duration("zero", 0),
		Duration("elapsed", 1500*time.Millisecond),
		Duration("negative", -time.Nanosecond),
		Duration("maximum", time.Duration(1<<63-1)),
		Duration("minimum", time.Duration(-1<<63)),
		Timestamp("occurred_at", timestamp),
		Timestamp("zero_time", time.Time{}),
		Secret("api_token", "top-secret-token-金木犀"),
	}

	t.Run("text", func(t *testing.T) {
		var output bytes.Buffer
		logger := mustTextLogger(t, &output, noTime(LevelInfo))
		if err := logger.Info("typed", fields); err != nil {
			t.Fatal(err)
		}
		want := "level=INFO message=\"typed\" zero=\"0s\" elapsed=\"1.5s\" negative=\"-1ns\"" +
			" maximum=\"2562047h47m16.854775807s\" minimum=\"-2562047h47m16.854775808s\"" +
			" occurred_at=\"2026-09-20T04:14:15.123456789Z\" zero_time=\"0001-01-01T00:00:00Z\"" +
			" api_token=\"[REDACTED]\"\n"
		if got := output.String(); got != want {
			t.Fatalf("text fields = %q, want %q", got, want)
		}
		if strings.Contains(output.String(), "top-secret-token-金木犀") {
			t.Fatalf("text output exposed secret: %q", output.String())
		}
	})

	t.Run("json", func(t *testing.T) {
		var output bytes.Buffer
		logger := mustJSONLogger(t, &output, noTime(LevelInfo))
		if err := logger.Info("typed", fields); err != nil {
			t.Fatal(err)
		}
		var record struct {
			Fields map[string]any `json:"fields"`
		}
		if err := stdjson.Unmarshal(output.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		want := map[string]string{
			"zero":        "0s",
			"elapsed":     "1.5s",
			"negative":    "-1ns",
			"maximum":     "2562047h47m16.854775807s",
			"minimum":     "-2562047h47m16.854775808s",
			"occurred_at": "2026-09-20T04:14:15.123456789Z",
			"zero_time":   "0001-01-01T00:00:00Z",
			"api_token":   "[REDACTED]",
		}
		for name, expected := range want {
			if got := record.Fields[name]; got != expected {
				t.Errorf("fields[%q] = %#v, want %q", name, got, expected)
			}
		}
		if strings.Contains(output.String(), "top-secret-token-金木犀") {
			t.Fatalf("JSON output exposed secret: %q", output.String())
		}
	})
}

type stringContractWriter struct {
	writeCalls       int
	writeStringCalls int
	delta            int
	err              error
}

func (writer *stringContractWriter) Write(value []byte) (int, error) {
	writer.writeCalls++
	return len(value), errors.New("Write must not be called")
}

func (writer *stringContractWriter) WriteString(value string) (int, error) {
	writer.writeStringCalls++
	return len(value) + writer.delta, writer.err
}

func TestStringWriterContractMatrix(t *testing.T) {
	tests := []struct {
		name    string
		delta   int
		err     error
		wantErr string
	}{
		{"exact", 0, nil, ""},
		{"short", -1, nil, io.ErrShortWrite.Error()},
		{"oversized", 1, nil, io.ErrShortWrite.Error()},
		{"error", 0, errors.New("string write failed"), "string write failed"},
	}
	for _, format := range []string{"text", "json"} {
		for _, test := range tests {
			t.Run(format+"_"+test.name, func(t *testing.T) {
				writer := &stringContractWriter{delta: test.delta, err: test.err}
				var logger *Logger
				if format == "json" {
					logger = mustJSONLogger(t, writer, noTime(LevelInfo))
				} else {
					logger = mustTextLogger(t, writer, noTime(LevelInfo))
				}
				err := logger.Info("message", nil)
				if test.wantErr == "" {
					if err != nil {
						t.Fatalf("Info() error = %v", err)
					}
				} else if err == nil || err.Error() != test.wantErr {
					t.Fatalf("Info() error = %v, want %q", err, test.wantErr)
				}
				if writer.writeCalls != 0 || writer.writeStringCalls != 1 {
					t.Fatalf("calls = Write:%d WriteString:%d", writer.writeCalls, writer.writeStringCalls)
				}
			})
		}
	}
}
