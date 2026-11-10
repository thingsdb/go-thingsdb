package thingsdb

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

func niceErr(err error) string {
	if err == io.EOF {
		return "Connection lost"
	}
	return err.Error()
}

var validNameRegex = regexp.MustCompile(`^[A-Za-z_][0-9A-Za-z_]{0,254}$`)

func isName(s string) bool {
	return validNameRegex.MatchString(s)
}

// CnScope returns the collection name for a collection scope.
func CnScope(scope string) (string, error) {
	var name string

	if strings.Contains(scope, ":") {
		parts := strings.Split(scope, ":")
		name = parts[len(parts)-1]
	} else if strings.Contains(scope, "/") {
		parts := strings.Split(scope, "/")
		name = parts[len(parts)-1]
	} else {
		name = ""
	}

	if isName(name) {
		return name, nil
	}

	return "", fmt.Errorf("invalid (collection) scope name: %s", scope)
}

// ToFullScope converts a collection scope to its full scope name.
func ToFullScope(scope string) string {
	if strings.HasPrefix(scope, "@collection:") {
		return scope
	}
	if cn, err := CnScope(scope); err == nil {
		return "@collection:" + cn
	}
	return scope // should not happed if a valid scope
}

// NilBytes represent a MessagePack byte array representing a simple `nil`.
var NilBytes = []byte{0xc0}
