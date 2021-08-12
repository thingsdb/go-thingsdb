package thingsdb

import (
	"github.com/vmihailenco/msgpack/v5"
)

// warnEvent is receveid when a warning is raised by ThingsDB
type warnEvent struct {
	Msg  string `msgpack:"warn_msg"`
	Code uint16 `msgpack:"warn_code"`
}

// newWarnEvent creates a new warning event
func newWarnEvent(pkg *pkg) (*warnEvent, error) {
	var result warnEvent
	err := msgpack.Unmarshal(pkg.data, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
