#!/usr/bin/env bash
set -euo pipefail

THRESHOLD=${1:-95}
COVER_PROFILE="coverage.out"

echo "Running Go tests with coverage (threshold: ${THRESHOLD}%) ..."
GOCACHE=${GOCACHE:-$(go env GOCACHE)} go test -coverprofile="${COVER_PROFILE}" -count=1 ./internal/...

echo ""
echo "=== Coverage by function ==="
go tool cover -func="${COVER_PROFILE}"

COVERAGE=$(go tool cover -func="${COVER_PROFILE}" | grep '^total:' | awk '{print substr($3, 1, length($3)-1)}')
echo ""
echo "Total coverage: ${COVERAGE}%  (threshold: ${THRESHOLD}%)"

if awk "BEGIN {exit (${COVERAGE} < ${THRESHOLD}) ? 0 : 1}"; then
    echo "FAIL: coverage ${COVERAGE}% is below the required ${THRESHOLD}%"
    exit 1
fi

echo "PASS: coverage meets the threshold."
