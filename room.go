package thingsdb

import (
	"fmt"
	"time"
)

// DefaultWait can be used as default time to wait for a Join
const DefaultWait = 60 * time.Second

func foo(room *Room) {}

// Room type can be used to join a ThingsDB room
type Room struct {
	// Private
	id            uint64
	code          *string
	scope         string
	conn          *Conn
	waitJoin      chan error
	eventHandlers map[string](func(room *Room, args []interface{}))

	// Public
	OnInit   func(room *Room)
	OnJoin   func(room *Room)
	OnLeave  func(room *Room)
	OnDelete func(room *Room)
	Data     interface{}
}

// NewRoom returns a Room by given code. The code should be ThingsDB code which return a Room Id. For example: `.myRoom.id();`
func NewRoom(scope string, code string) *Room {
	room := NewRoomFromId(scope, 0)
	room.code = &code
	return room
}

// NewRoomFromId returns a Room by a Scope and Id
func NewRoomFromId(scope string, id uint64) *Room {
	return &Room{
		// Private
		id:            id,
		code:          nil,
		scope:         scope,
		conn:          nil,
		waitJoin:      nil,
		eventHandlers: make(map[string]func(room *Room, args []interface{})),

		// Public
		OnInit:   foo,
		OnJoin:   foo,
		OnLeave:  foo,
		OnDelete: foo,
		Data:     nil,
	}
}

// Id returns the Room Id (0 when the room Id is not resolved yet)
func (room *Room) Id() uint64 {
	return room.id
}

// Scope returns the Room Scope
func (room *Room) Scope() string {
	return room.scope
}

// HandleEvent can be used to add an event handler
func (room *Room) HandleEvent(event string, handle func(room *Room, args []interface{})) {
	room.eventHandlers[event] = handle
}

// Join must be called to actually join the Room
func (room *Room) Join(conn *Conn, wait time.Duration) error {

	if wait > 0 {
		room.waitJoin = make(chan error)
	}

	room.conn = conn

	err := room.join(conn)
	if err != nil {
		return err
	}

	if wait > 0 {

		go func() {
			time.Sleep(wait)
			room.waitJoin <- fmt.Errorf("Timeout while waiting for the join event on room Id %d", room.id)
		}()

		for {
			select {
			case err := <-room.waitJoin:
				room.waitJoin = nil
				return err
			}
		}
	}
	return nil
}

// Leave can be used to leave a room
func (room *Room) Leave() error {
	if room.id == 0 {
		return fmt.Errorf("Room Id is zero (0), most likely the room has never been joined")
	}

	if room.conn == nil {
		return fmt.Errorf("Room Id %d is not joined", room.id)
	}

	roomIds := []*uint64{&room.id}
	err := room.conn.leave(room.scope, roomIds)
	if err != nil {
		return err
	}

	if roomIds[0] == nil {
		return fmt.Errorf("Room Id %d not found (anymore)", room.id)
	}

	return nil
}

// Emit can be used to emit an event to the room
// Note: `args` should be an array with positional arguments or nil
func (room *Room) Emit(event string, args []interface{}) error {
	if room.conn == nil {
		return fmt.Errorf("Room Id %d is not joined", room.id)
	}
	return room.conn.Emit(room.scope, room.id, event, args)
}

func (room *Room) join(conn *Conn) error {
	conn.rooms.mux.Lock()
	defer conn.rooms.mux.Unlock()

	if room.id == 0 {
		if room.code == nil {
			return fmt.Errorf("Code or a room Id > 0 is required")
		}
		val, err := conn.Query(room.scope, *room.code, nil)
		if err != nil {
			return err
		}

		var roomId uint64

		switch val.(type) {
		case int:
			roomId = uint64(val.(int))
		case int8:
			roomId = uint64(val.(int8))
		case int16:
			roomId = uint64(val.(int16))
		case int32:
			roomId = uint64(val.(int32))
		case int64:
			roomId = uint64(val.(int64))
		case uint8:
			roomId = uint64(val.(uint8))
		case uint16:
			roomId = uint64(val.(uint16))
		case uint32:
			roomId = uint64(val.(uint32))
		case uint64:
			roomId = val.(uint64)
		default:
			return fmt.Errorf("Expecting code `%s` to return with a room Id (type integer), bot got: %v", *room.code, val)
		}

		roomIds := []*uint64{&roomId}
		err = conn.join(room.scope, roomIds)
		if err != nil {
			return err
		}

		if roomIds[0] == nil {
			return fmt.Errorf("Room Id %d not found. The Id was returned using ThingsDB code: %s", roomId, *room.code)
		}

		room.id = roomId
	} else {
		roomIds := []*uint64{&room.id}
		err := conn.join(room.scope, roomIds)
		if err != nil {
			return err
		}

		if roomIds[0] == nil {
			return fmt.Errorf("Room Id %d not found", room.id)
		}
	}

	conn.rooms.store[room.id] = room
	room.OnInit(room)

	return nil
}

func (room *Room) onStop(f func(room *Room)) {
	delete(room.conn.rooms.store, room.id)
	f(room)
}

func (room *Room) onEvent(ev *roomEvent) {
	switch ev.Tp {
	case ProtoOnRoomJoin:
		room.OnJoin(room)
		if room.waitJoin != nil {
			room.waitJoin <- nil
		}
	case ProtoOnRoomLeave:
		room.onStop(room.OnLeave)
	case ProtoOnRoomDelete:
		room.onStop(room.OnDelete)
	case ProtoOnRoomEvent:
		if f, ok := room.eventHandlers[ev.Event]; ok {
			f(room, ev.Args)
		} else {
			room.conn.logDebug("No handler for event: %s", ev.Event)
		}
	}
}
