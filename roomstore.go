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

func (rs *roomStore) getRoomMap() map[string][]*uint64 {
	roomMap := make(map[string][]*uint64)
	rs.mux.Lock()
	for roomId, room := range rs.store {
		roomMap[room.scope] = append(roomMap[room.scope], &roomId)
	}
	rs.mux.Unlock()
	return roomMap
}
