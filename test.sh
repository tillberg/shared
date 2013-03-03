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

mkdir -p /tmp/a
mkdir -p /tmp/b
rm -rf /tmp/a/.git
rm -rf /tmp/b/.git
mv /tmp/cache1 /tmp/a/.git
mv /tmp/cache2 /tmp/b/.git
echo "ref: refs/heads/master" > /tmp/a/.git/HEAD
echo "ref: refs/heads/master" > /tmp/b/.git/HEAD
mkdir -p /tmp/a/.git/refs/heads/
mkdir -p /tmp/b/.git/refs/heads/
