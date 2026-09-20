# Test matrix

| Area | Normal cases | Edge cases | Failure cases |
| --- | --- | --- | --- |
| Levels | all 4 minimum levels × all 4 event levels | exact threshold boundaries, dynamic `log` | filtered records produce no output or validation/writer side effects |
| Field kinds | text, int, int64, number, boolean, error, duration, timestamp, secret | Unicode/empty values, typed-nil errors, zero/min/max durations, timezone normalization, zero time | non-finite JSON numbers; source secret is never retained or emitted |
| Field names | ASCII letters, digits, `_`, `.`, `-` | shortest names, metadata-like names | empty, invalid first byte, spaces, controls, separators, Unicode |
| Duplicate fields | unique record and persistent fields | metadata names remain isolated under `fields` | record, persistent, and persistent/record collisions |
| Groups | empty, single, nested, snapshotted groups | caller mutation after construction | invalid group/child names, duplicates, unsupported child values |
| Formats | text and JSON | quotes, slashes, controls, invalid UTF-8, optional RFC3339Nano UTC time | malformed/partial JSON must never be emitted |
| Context | persistent fields and child loggers | root/child independence, multiple layers | invalid/duplicate context fails at child creation; caller slice mutation must not alter logger |
| Writers | exact `Write` and `WriteString` paths | short, negative, and oversized counts | nil/typed-nil rejection; early/late writer errors are returned before short-write checks |
| Concurrency | shared JSON logger and related text child loggers | 800 JSON and 400 text records | no interleaved or malformed lines; race detector clean |
| Robustness | fixed regression inputs and static analysis | arbitrary messages, names, values, groups, and secrets | four fuzz targets never panic, emit malformed JSON, or expose secret values |
