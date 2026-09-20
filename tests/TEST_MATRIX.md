# Test matrix

| Area | Normal cases | Edge cases | Failure cases |
| --- | --- | --- | --- |
| Levels | debug, info, warn, error, generic log | exact minimum boundary | filtered records produce no output |
| Fields | text, int, int64, number, boolean, error | Unicode, empty names/values, nil errors, numeric extremes | non-finite JSON numbers |
| Groups | single and nested groups | empty groups | invalid child values |
| Formats | text and JSON | escaping, optional RFC3339Nano UTC time | malformed JSON must never be emitted |
| Context | persistent fields and child loggers | root/child independence, multiple layers | caller slice mutation must not alter logger |
| Writers | complete writes | zero/short writes | writer errors are returned |
| Concurrency | shared child logger | 800 concurrent records | no interleaved or malformed lines; race detector clean |
| Robustness | fixed regression inputs | arbitrary UTF-8 and byte strings | text/JSON fuzz targets never panic |
