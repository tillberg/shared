#!/usr/bin/env bash
source ./setup.sh
while :
do
    go run src/shared.go --watch _sync$1 --cache _cache$1 --port 925$1
    sleep 0.5
done
