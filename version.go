package thingsdb

// Version exposes the go-thingsdb version
const Version = "1.0.8"

//Publish module:
//
//  go mod tidy
//  TI_TOKEN="Fai6N..." go test ./...
//  git tag {VERSION}
//  git push origin {VERSION}
//  GOPROXY=proxy.golang.org go list -m github.com/thingsdb/go-thingsdb@{VERSION}
