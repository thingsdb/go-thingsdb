package thingsdb

type Room struct {
	id    uint64
	code  *string
	scope *string
	conn  *Conn
}

func newRoomFromId(id uint64, scope *string) *Room {
	return &Room{
		id:    id,
		code:  nil,
		scope: scope,
	}
}

func newRoom(Code string, scope *string) *Room {
	return &Room{
		id:    0,
		code:  nil,
		scope: scope,
	}
}

func (room *Room) Join(conn *Conn) {
	conn.rooms.registerRoom(room)
}
