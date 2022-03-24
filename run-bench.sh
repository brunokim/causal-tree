#!/bin/bash

OUTPUT="${1}"
if [ -z "${OUTPUT}" ]; then
    OUTPUT=bench/$(git rev-parse --short HEAD).log
fi

if [ ! -z "$(git status --porcelain)" ]; then
    echo "Warning: git tree is dirty"
    if [ -e "${OUTPUT}" ]; then
        echo "
Won't append to already existing file ${OUTPUT} with a dirty tree.
You may provide a different output as argument to this script"
        exit 1
    fi
fi

go1.18 test ./... -v -bench=. -run=TestXXX | tee -a "${OUTPUT}"
