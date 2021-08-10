package thingsdb

import "io"

func niceErr(err error) string {
	if err == io.EOF {
		return "Connection lost"
	}
	return err.Error()
}
