package thingsdb

import (
	"github.com/vmihailenco/msgpack/v5"
)

// roomEvent is an event dedicated to the rooms API (Join/Leave/Emit/Delete)
type roomEvent struct {
	Tp    Proto
	Id    uint64        `msgpack:"id"`
	Event string        `msgpack:"event"`
	Args  []interface{} `msgpack:"args"`
}

// newRoomEvent creates a new node status
func newRoomEvent(pkg *pkg) (*roomEvent, error) {

	var result roomEvent
	err := msgpack.Unmarshal(pkg.data, &result)
	if err != nil {
		return nil, err
	}

	result.Tp = Proto(pkg.tp)

	return &result, nil
}
