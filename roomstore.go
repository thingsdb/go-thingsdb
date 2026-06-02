package thingsdb

import "sync"

type roomStore struct {
	store map[string]map[uint64]*Room
	mux   sync.Mutex
}

func newRoomStore() *roomStore {
	return &roomStore{
		store: make(map[string]map[uint64]*Room),
	}
}

func (rs *roomStore) getRoom(scope string, id uint64) (*Room, bool) {
	rs.mux.Lock()
	room, ok := rs.store[scope][id]
	if !ok && scope == "" {
		// fallback for ThinsDB < 1.8.6
		for _, v := range rs.store {
			room, ok = v[id]
			if ok {
				break
			}
		}
	}
	rs.mux.Unlock()
	return room, ok
}

func (rs *roomStore) getRoomMap() map[string][]*uint64 {
	roomMap := make(map[string][]*uint64)
	rs.mux.Lock()
	for _, rooms := range rs.store {
		for roomId, room := range rooms {
			roomMap[room.scope] = append(roomMap[room.scope], &roomId)
		}
	}
	rs.mux.Unlock()
	return roomMap
}
