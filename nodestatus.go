package thingsdb

import (
	"github.com/vmihailenco/msgpack/v5"
)

// nodeStatus is used for the node status event
type nodeStatus struct {
	Id     uint32 `msgpack:"id"`
	Status string `msgpack:"status"`
}

// newNodeStatus creates a new node status
func newNodeStatus(pkg *pkg) (*nodeStatus, error) {

	var result nodeStatus
	err := msgpack.Unmarshal(pkg.data, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
