#!/usr/bin/env bash
set -uo pipefail

if (($# < 2)); then
  echo "Usage: quiet-run.sh LABEL COMMAND [ARG...]" >&2
  exit 2
fi

label=$1
shift
output=$(mktemp) || exit 1
trap 'rm -f -- "$output"' EXIT

printf '%s ...\n' "$label"
if "$@" >"$output" 2>&1; then
  printf '%s: passed\n' "$label"
else
  status=$?
  printf '%s: failed (exit %s); full output follows\n' "$label" "$status" >&2
  cat "$output" >&2
  exit "$status"
fi
