#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT HUP INT TERM

keika_command=${KEIKA:-keika}
go_cache="$test_dir/go-cache"
generated_dir="$test_dir/generated"
mkdir -p "$go_cache" "$generated_dir"

cd "$project_dir"
"$keika_command" deps lock
GOCACHE="${GOCACHE:-$test_dir/go-cache}" go test "$project_dir/scripts/check_coverage.go" "$project_dir/scripts/check_coverage_test.go"
"$keika_command" check log.km
"$keika_command" check examples/basic/main.km
"$keika_command" check examples/json/main.km
"$keika_command" emit-go -package log -o "$generated_dir/log.go" -source-map "$generated_dir/log.map.json" log.km
cp tests/*_test.go "$generated_dir/"

cd "$generated_dir"
go mod init example.com/kinmokusei-log-test >/dev/null
GOCACHE="$go_cache" go vet ./...
GOCACHE="$go_cache" go test -count=1 -race -coverprofile=coverage.out ./...
GOCACHE="$go_cache" go run "$project_dir/scripts/check_coverage.go" -- "$generated_dir/log.go" "$generated_dir/log.map.json" "$generated_dir/coverage.out"
GOCACHE="$go_cache" go test -run='^$' -fuzz='^FuzzJSONLoggingNeverPanicsAndStaysValid$' -fuzztime=1s
GOCACHE="$go_cache" go test -run='^$' -fuzz='^FuzzTextLoggingNeverPanics$' -fuzztime=1s
GOCACHE="$go_cache" go test -run='^$' -fuzz='^FuzzGroupValidationNeverPanics$' -fuzztime=1s
GOCACHE="$go_cache" go test -run='^$' -fuzz='^FuzzSecretAlwaysRedacted$' -fuzztime=1s

cd "$project_dir"
"$keika_command" build -o "$test_dir/basic-example" examples/basic/main.km
"$keika_command" build -o "$test_dir/json-example" examples/json/main.km

actual_output=$("$test_dir/basic-example")
expected_output='level=INFO message="request completed" component="api" request={method="GET" status=200} cached=true'
if [ "$actual_output" != "$expected_output" ]; then
  printf 'unexpected basic example output:\n%s\n' "$actual_output" >&2
  exit 1
fi

actual_output=$("$test_dir/json-example")
expected_output='{"level":"INFO","message":"worker ready","fields":{"queue":"emails","workers":4}}'
if [ "$actual_output" != "$expected_output" ]; then
  printf 'unexpected JSON example output:\n%s\n' "$actual_output" >&2
  exit 1
fi

library_dir="$test_dir/library"
consumer_dir="$test_dir/external-consumer"
mkdir "$library_dir"
cp log.km go.mod kinmokusei.lock kinmokusei.toml LICENSE "$library_dir/"
"$keika_command" new app --module example.com/kinmokusei-log-consumer "$consumer_dir"
cp tests/external_consumer/main.km "$consumer_dir/main.km"
cd "$consumer_dir"
"$keika_command" deps add --offline --replace ../library github.com/puffball1567/kinmokusei-log@v0.2.1
"$keika_command" check
"$keika_command" build -o "$test_dir/external-consumer-app"
actual_output=$("$test_dir/external-consumer-app")
expected_output='{"level":"INFO","message":"started","fields":{"service":"consumer","ready":true}}'
if [ "$actual_output" != "$expected_output" ]; then
  printf 'unexpected external-consumer output:\n%s\n' "$actual_output" >&2
  exit 1
fi
