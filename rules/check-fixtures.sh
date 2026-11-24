#!/bin/sh
# Checks what the ruleguard rules in rules/*.go report.
#
# 1. Lints rules/testdata/fixture with gocritic and the repository's
#    .golangci.yaml. A fixture line that ends in // want "regexp" must get a
#    report matching the regexp (POSIX ERE, no double quote inside; use .
#    for one); every other line must get none.
# 2. Lints again with a copy of .golangci.yaml that also loads
#    rules/testdata/broken/broken.go, which ruleguard cannot load, and expects
#    the run to fail with a ruleguard init error (failOn: all).
#
# Run it from anywhere with the golangci-lint version CI pins on PATH:
#
#	sh rules/check-fixtures.sh
#
# Each run uses a fresh lint cache, since the cache does not track rules/*.go.
set -eu

cd "$(dirname "$0")/.."

fixture=rules/testdata/fixture
broken=rules/testdata/broken/broken.go

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
GOLANGCI_LINT_CACHE=$tmp/cache
export GOLANGCI_LINT_CACHE

lint() {
	golangci-lint run --allow-parallel-runners --enable-only gocritic \
		--issues-exit-code 0 --show-stats=false \
		--output.text.path stdout --output.text.print-issued-lines=false \
		--output.text.print-linter-name=false --output.text.colors=false "$@"
}

unformatted=$(gofmt -l rules/testdata)
if [ -n "$unformatted" ]; then
	echo "check-fixtures: not gofmt-formatted: $unformatted" >&2
	exit 1
fi

status=0
lint "./$fixture" >"$tmp/reports" 2>"$tmp/stderr" || status=$?
if [ "$status" -ne 0 ]; then
	echo "check-fixtures: golangci-lint exited $status on $fixture" >&2
	cat "$tmp/stderr" "$tmp/reports" >&2
	exit 1
fi

# Marks: "path:line<TAB>regexp", one per want comment.
awk '
	match($0, /\/\/ want "[^"]*"[ \t]*$/) {
		s = substr($0, RSTART + 9)
		sub(/"[ \t]*$/, "", s)
		printf "%s:%d\t%s\n", FILENAME, FNR, s
	}
' "$fixture"/*.go >"$tmp/marks"

if [ ! -s "$tmp/marks" ]; then
	echo "check-fixtures: no want comments in $fixture" >&2
	exit 1
fi

# Reports: "path:line:col: message". Compare both ways.
awk -F '\t' '
	FNR == NR { want[$1] = $2; next }
	match($0, /^[^:]+:[0-9]+:[0-9]+: /) {
		head = substr($0, 1, RLENGTH - 2)
		msg = substr($0, RLENGTH + 1)
		sub(/:[0-9]+$/, "", head)
		seen[head] = 1
		if (!(head in want)) {
			print "unexpected report: " $0
			bad = 1
		} else if (msg !~ want[head]) {
			print "report does not match want \"" want[head] "\": " $0
			bad = 1
		}
	}
	END {
		for (k in want) {
			if (!(k in seen)) {
				print "missing report: " k ": want \"" want[k] "\""
				bad = 1
			}
		}
		exit bad
	}
' "$tmp/marks" "$tmp/reports" >&2 || {
	echo "check-fixtures: reports differ from the want comments in $fixture" >&2
	exit 1
}

# Load failure: the same config, with the broken rules file added.
sed 's#^\( *rules: \)"rules/\*\.go"$#\1"rules/*.go,'"$broken"'"#' \
	.golangci.yaml >"$tmp/golangci.yaml"
if ! grep -q "rules: \"rules/\*\.go,$broken\"" "$tmp/golangci.yaml"; then
	echo "check-fixtures: .golangci.yaml has no ruleguard line rules: \"rules/*.go\"" >&2
	exit 1
fi

GOLANGCI_LINT_CACHE=$tmp/cache-broken
status=0
lint --config "$tmp/golangci.yaml" "./$fixture" >"$tmp/broken" 2>&1 || status=$?
if [ "$status" -eq 0 ] || ! grep -q "ruleguard init error: $broken:" "$tmp/broken"; then
	echo "check-fixtures: lint did not fail on $broken (exit $status); is failOn: all set?" >&2
	cat "$tmp/broken" >&2
	exit 1
fi

echo "check-fixtures: ok ($(wc -l <"$tmp/marks" | tr -d ' ') reports, load failure detected)"
