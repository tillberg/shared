set -e
echo -n "Compiling..."
go get code.google.com/p/goprotobuf/{proto,protoc-gen-go}
mkdir -p src/sharedpb
protoc -I=proto/ --go_out=src/sharedpb/ proto/shared.proto
echo -n "."
go build src/shared.go
echo " done."
set +e
go test src/shared_test.go -parallel 1 $*

rm -rf /tmp/.git
mv /tmp/cache1 /tmp/.git
echo "ref: refs/heads/master" > /tmp/.git/HEAD
mkdir -p /tmp/.git/refs/heads/
