#!/usr/bin/env bash

GOPATH=~/tmp/go/shared/; export GOPATH; mkdir -p $GOPATH
PATH=$GOPATH/bin:$PATH

go get github.com/golang/protobuf/{proto,protoc-gen-go}
go get github.com/howeyc/fsnotify
go get github.com/tillberg/goconfig
mkdir -p src/sharedpb
protoc -I=proto/ --go_out=src/sharedpb/ proto/shared.proto
