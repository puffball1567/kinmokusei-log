# kinmokusei-log

`kinmokusei-log` is a structured logging library written for Kinmokusei. It
provides typed fields, text and JSON output, level filtering, persistent
context, explicit write errors, and concurrency-safe records without external
runtime dependencies.

## Requirements

- Kinmokusei v0.4.3 or later
- Go 1.23 or later

## Features

- Debug, info, warning, and error levels
- Minimum-level filtering before encoding or writing
- Human-readable text and machine-readable JSON output
- Text, integer, 64-bit integer, floating-point, boolean, and error fields
- Nested structured groups
- Persistent fields through independent child loggers
- Optional UTC timestamps in RFC3339Nano format
- Complete-record locking for safe concurrent writes
- Writer, short-write, and JSON encoding errors returned as `Result<void>`
- No implicit process exit for error-level logs

JSON records keep user fields under a `fields` object. This prevents a caller
field named `message`, `level`, or `time` from replacing record metadata.

## Installation

```sh
keika deps add github.com/puffball1567/kinmokusei-log@v0.1.0
```

```ts
import { Boolean, Level, Options, Text, json } from "kinmokusei-log";
```

## Example

```ts
import { Boolean, Integer, Level, Options, Text, json } from "kinmokusei-log";
import go fmt from "fmt";
import go os from "os";

function main(): void {
  const [logger, loggerError] = json(os.Stdout, Options {
    minimumLevel: Level.Info,
    includeTime: true,
  });
  if (loggerError !== nil) {
    fmt.Println("error:", loggerError.Error());
    return;
  }

  const service = logger.withFields([Text("service", "worker")]);
  const [writeError] = service.info("job completed", [
    Integer("job_id", 42),
    Boolean("cached", false),
  ]);
  if (writeError !== nil) {
    fmt.Println("error:", writeError.Error());
  }
}
```

The output is one JSON object per line:

```json
{"time":"2026-09-20T04:30:00Z","level":"INFO","message":"job completed","fields":{"service":"worker","job_id":42,"cached":false}}
```

## Field groups

`Group` validates and snapshots its children when it is created. It returns a
`Result<Field>` because JSON cannot represent non-finite floating-point values.

```ts
const request = Group("request", [
  Text("method", "GET"),
  Integer("status", 200),
])?;

logger.info("request completed", [request])?;
```

Groups are nested JSON objects. In text output they use a compact
`request={method="GET" status=200}` representation.

## Logger behavior

- `enabled(level)` reports whether a record would pass the minimum level.
- `log(level, message, fields)` handles a dynamic level.
- `debug`, `info`, `warn`, and `error` are convenience methods.
- `withFields(fields)` creates an independent child logger. It shares the
  writer lock while keeping the parent context unchanged.
- Filtered records are not encoded and do not touch the writer.
- Every emitted record is assembled before the lock is taken and written with
  one writer call. Multiple related loggers can therefore safely share output.
- Error-level logging does not terminate the process. Application lifecycle
  decisions stay with the application.

## Development

Run source checks, 100% generated-Go statement coverage, the race detector,
two fuzz targets, both examples, and an independent short-import consumer:

```sh
KEIKA=keika sh scripts/test.sh
```

Generated Go is test output and is not maintained as source.

## License

Apache-2.0
