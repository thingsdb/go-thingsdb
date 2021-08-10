package thingsdb

import (
	"github.com/vmihailenco/msgpack/v5"
)

// NodeStatus is writting to the client event channel in case of an event
type NodeStatus struct {
	Id     uint32 `msgpack:"id"`
	Status string `msgpack:"status"`
}

// newNodeStatus creates a new node status
func newNodeStatus(pkg *pkg) (*NodeStatus, error) {

	var result NodeStatus
	err := msgpack.Unmarshal(pkg.data, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
