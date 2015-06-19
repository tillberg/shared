#!/usr/bin/env bash

set -e
echo -n "Compiling..."
source ./setup.sh
echo -n "."
go build src/shared.go
echo " done."
set +e
go test src/shared_test.go -parallel 1 $*

TEST_ROOT=/tmp # .../shared_test
mkdir -p $TEST_ROOT/a
mkdir -p $TEST_ROOT/b
rm -rf $TEST_ROOT/a/.git
rm -rf $TEST_ROOT/b/.git
mv $TEST_ROOT/cache1 $TEST_ROOT/a/.git
mv $TEST_ROOT/cache2 $TEST_ROOT/b/.git
echo "ref: refs/heads/master" > $TEST_ROOT/a/.git/HEAD
echo "ref: refs/heads/master" > $TEST_ROOT/b/.git/HEAD
mkdir -p $TEST_ROOT/a/.git/refs/heads/
mkdir -p $TEST_ROOT/b/.git/refs/heads/
