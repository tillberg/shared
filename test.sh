set -e
go get code.google.com/p/goprotobuf/{proto,protoc-gen-go}
mkdir -p src/sharedpb
protoc -I=proto/ --go_out=src/sharedpb/ proto/shared.proto

go build src/shared.go
go test src/shared_test.go -v -parallel 1 $*
