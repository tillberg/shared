set -e
echo -n "Compiling..."
go get code.google.com/p/goprotobuf/{proto,protoc-gen-go}
mkdir -p src/sharedpb
protoc -I=proto/ --go_out=src/sharedpb/ proto/shared.proto
echo -n "."
go build src/shared.go
echo " done."
go test src/shared_test.go -parallel 1 $*
