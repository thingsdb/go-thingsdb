package thingsdb

import "sync"

type roomStore struct {
	store map[uint64]*Room
	mux   sync.Mutex
}

func newRoomStore() *roomStore {
	return &roomStore{
		store: make(map[uint64]*Room),
	}
}

func (rs *roomStore) getRoom(id uint64) (*Room, bool) {
	rs.mux.Lock()
	room, ok := rs.store[id]
	rs.mux.Unlock()
	return room, ok
}
