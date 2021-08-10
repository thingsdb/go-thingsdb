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

func (rs *roomStore) getRoom(id uint64) *Room {
	rs.mux.Lock()
	room := rs.store[id]
	rs.mux.Unlock()
	return room
}

func (rs *roomStore) registerRoom(room *Room) {
	rs.mux.Lock()
	rs.store[room.id] = room
	rs.mux.Unlock()
}
