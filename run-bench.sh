#!/bin/bash

OUTPUT="${1}"
COUNT="${2}"

if [ -z "${OUTPUT}" ]; then
    OUTPUT=bench/$(git rev-parse --short HEAD)
fi

EXTRA_FLAGS=
if [ ! -z "${COUNT}" ]; then
    EXTRA_FLAGS="-count=${COUNT}"
fi

if [ ! -z "$(git status --porcelain)" ]; then
    echo -e "WARNING: git tree is dirty\n"
    if [ -e "${OUTPUT}.log" ]; then
        echo "
Won't append to already existing file ${OUTPUT}.log with a dirty tree.
You may provide a different output as argument to this script"
        exit 1
    fi
fi

# Echo command (set -x) for a specific one: https://unix.stackexchange.com/a/177911/420855
# Get first error code of a pipe: https://unix.stackexchange.com/a/73180/420855
TMPOUTPUT=$(mktemp)
( set -x -o pipefail; go1.18 test ./... -v -bench=. -run=TestXXX ${EXTRA_FLAGS} | tee -a "${TMPOUTPUT}" )
if [ $? -ne 0 ]; then
    exit
else
    cat "${TMPOUTPUT}" >> "${OUTPUT}.log"
fi

if ! command -v benchstat &>/dev/null; then
    echo "
benchstat is not installed. You can install it with

go install golang.org/x/perf/cmd/benchstat"
else
    (set -x; benchstat -csv "${OUTPUT}.log" > "${OUTPUT}.csv")
    echo "Stat analysis written to ${OUTPUT}.csv"
fi
