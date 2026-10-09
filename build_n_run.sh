#!/bin/sh
# Build and run the Go port. Extra args go to the game (e.g. -load EOBDATA.SAV).
set -e
cd "$(dirname "$0")"
DATA="${EOB_DATA:-$PWD/../Eye of the Beholder.app/Contents/Resources/game}"
go build -o bin/eob ./cmd/eob
exec bin/eob -data "$DATA" "$@"
