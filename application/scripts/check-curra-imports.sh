#!/bin/bash
# CI boundary check: verifies only internal/curra imports CURRA solver packages.
# This enforces the frozen architecture rule that CURRA internals are only
# accessible through the adapter boundary.

set -e

echo "Checking CURRA import boundaries..."

# Find all Go files outside internal/curra/ that import CURRA solver packages
VIOLATIONS=$(grep -r "github.com/sPreetham42/timetable-platform/internal/scheduler" \
  --include="*.go" \
  application/ \
  | grep -v "application/internal/curra/" \
  | grep -v "_test.go" || true)

if [ -n "$VIOLATIONS" ]; then
  echo "ERROR: CURRA import boundary violation detected!"
  echo ""
  echo "The following files import CURRA packages outside the adapter boundary:"
  echo "$VIOLATIONS"
  echo ""
  echo "Only application/internal/curra/ is permitted to import CURRA packages."
  exit 1
fi

echo "CURRA import boundary check passed."
