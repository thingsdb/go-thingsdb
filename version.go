package thingsdb

//Version exposes the go-thingsdb version
const Version = "1.0.1"

//Publish module:
//
//  go mod tidy
//  go test ./...
//  git tag {VERSION}
//  git push origin {VERSION}
//  GOPROXY=proxy.golang.org go list -m https://github.com/thingsdb/go-thingsdb@{VERSION}
