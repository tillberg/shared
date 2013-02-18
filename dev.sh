go get code.google.com/p/goprotobuf/{proto,protoc-gen-go}
mkdir -p sharedpb
protoc -I=proto/ --go_out=sharedpb/ proto/shared.proto
while :
do
    go run shared.go
    sleep 0.5
done
