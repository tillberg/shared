set -e
go build src/shared.go
go test src/shared_test.go -v -parallel 1 $*
