package thingsdb

// Version exposes the go-thingsdb version
const Version = "1.2.5"

//Publish module:
//
//  go get -u ./...
//  go mod tidy
//  TI_TOKEN="Fai6N..." go test ./...
//  git tag {VERSION}
//  git push origin {VERSION}
//  GOPROXY=proxy.golang.org go list -m github.com/thingsdb/go-thingsdb@{VERSION}
