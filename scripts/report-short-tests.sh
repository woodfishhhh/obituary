#!/usr/bin/env bash
# Report only: gotestsum's slowest command succeeds even when rows are found.
# Input: go test -short -json ./... > short.json
set -euo pipefail
threshold=2s
go run gotest.tools/gotestsum@v1.13.0 tool slowest \
  --jsonfile "${1:?path to the short test JSON log}" --threshold "$threshold"
