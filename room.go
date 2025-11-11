package thingsdb

import (
	"fmt"
	"time"
)

// DefaultWait can be used as default time to wait for a Join
const DefaultWait = 60 * time.Second

func foo(room *Room)                                   {}
func bar(room *Room, event string, args []interface{}) {}

// Room type can be used to join a ThingsDB room
type Room struct {
	// Private
	id            uint64
	code          *string
	name          *string
	scope         string
	conn          *Conn
	waitJoin      chan error
	eventHandlers map[string](func(room *Room, args []interface{}))

	// Public
	// Note: OnEmit will *only* be called when no event handler for the given
	//       even is implemented.
	OnInit   func(room *Room)
	OnJoin   func(room *Room)
	OnLeave  func(room *Room)
	OnDelete func(room *Room)
	OnEmit   func(room *Room, event string, args []interface{})
	Data     interface{}
}

// NewRoom creates a new room using code. The code should return the room Id for the room.
//
// Example:
//
//	// Suppose Collection stuff has a room (.room)
//	room := thingsdb.NewRoom("//stuff", ".room.id();")
func NewRoom(scope string, code string) *Room {
	room := NewRoomFromId(scope, 0)
	room.code = &code
	return room
}

// NewRoomFromId creates a new room using a room Id.
//
// If the room Id unknown, you may want use `NewRoom(..)` to get the Id for the room by code.
//
// Example:
//
//	// Suppose Collection stuff has a room with Id 17
//	room := thingsdb.NewRoomFromId("//stuff", 17)
func NewRoomFromId(scope string, id uint64) *Room {
	return &Room{
		// Private
		id:            id,
		code:          nil,
		name:          nil,
		scope:         scope,
		conn:          nil,
		waitJoin:      nil,
		eventHandlers: make(map[string]func(room *Room, args []interface{})),

		// Public
		OnInit:   foo,
		OnJoin:   foo,
		OnLeave:  foo,
		OnDelete: foo,
		OnEmit:   bar,
		Data:     nil,
	}
}

// NewRoom creates a new room using the room name.
//
// Example:
//
//	// Suppose Collection stuff has a room (.room)
//	room := thingsdb.NewRoomFromName("//stuff", "my_room")
func NewRoomFromName(scope string, name string) *Room {
	room := NewRoomFromId(scope, 0)
	code := "room(name).id();"
	room.code = &code
	room.name = &name
	return room
}

// Id returns the Id of the room.
//
// > Note: Id() returns `0` when the room was created using `NewRoom(..)` and the room has never been joined.
func (room *Room) Id() uint64 {
	return room.id
}

// Scope returns the Room Scope
func (room *Room) Scope() string {
	return room.scope
}

// HandleEvent adds an event handler to the room.
//
// Example:
//
//	func onNewMessage(room *thingsdb.Room, args []interface{}) {
//	    if len(args) != 1 {
//	       fmt.Println("Invalid number of arguments")
//	       return
//	    }
//
//	    msg, ok := args[0].(string)
//	    if !ok {
//	       fmt.Println("Expecting argument 1 to be of type string")
//	       return
//	    }
//
//	    fmt.Println(msg)  // Just print the message
//	}
//
//	room = thingsdb.NewRoom("//stuff", ".chatRoom.id();")
//
//	// Add event handler for the "new-message" event
//	room.HandleEvent("new-message", onNewMessage)
func (room *Room) HandleEvent(event string, handle func(room *Room, args []interface{})) {
	room.eventHandlers[event] = handle
}

// Join must be called to actually join the room.
//
// The `wait` argument may be set to `0` to tell the room not to wait for the join to complete.
// If `wait` is set to any other positive value, then both the `OnInit` and `OnJoin` are
// called (in this order) before the call to Join returns unless the `OnJoin` is not completed
// before the `wait` duration (an error will be returned).
//
// Example:
//
//	err := room.Join(conn, thingsdb.DefaultWait)
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
			room.waitJoin <- fmt.Errorf("timeout while waiting for the join event on room Id %d", room.id)
		}()

		for {
			err := <-room.waitJoin
			room.waitJoin = nil
			return err
		}
	}

	return nil
}

// Leave will stop listening for events on a room.
//
// Example:
//
//	err := room.Leave()
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

// Emit an even to a room.
//
// Example:
//
//	args := []interface{}{"Just some chat message"}
//
//	err := room.Emit(
//	    "new-message",  // Event to emit
//	    args            // Arguments (may be nil)
//	);
func (room *Room) Emit(event string, args []interface{}) error {
	if room.conn == nil {
		return fmt.Errorf("Room Id %d is not joined", room.id)
	}
	return room.conn.Emit(room.scope, room.id, event, args)
}

// Emit an even to a room to peers only (no echo back).
//
// Example:
//
//	args := []interface{}{"Just some chat message"}
//
//	err := room.EmitPeers(
//	    "new-message",  // Event to emit to peers
//	    args            // Arguments (may be nil)
//	);
func (room *Room) EmitPeers(event string, args []interface{}) error {
	if room.conn == nil {
		return fmt.Errorf("Room Id %d is not joined", room.id)
	}
	return room.conn.EmitPeers(room.scope, room.id, event, args)
}

func (room *Room) join(conn *Conn) error {
	conn.rooms.mux.Lock()
	defer conn.rooms.mux.Unlock()

	if room.id == 0 {
		var vars map[string]interface{}
		if room.code == nil {
			return fmt.Errorf("Code or a room Id > 0 is required")
		}
		if room.name != nil {
			vars = map[string]interface{}{
				"name": *room.name,
			}
		}
		val, err := conn.Query(room.scope, *room.code, vars)
		if err != nil {
			return err
		}

		var roomId uint64

		switch val := val.(type) {
		case int:
		case int8:
		case int16:
		case int32:
		case int64:
		case uint8:
		case uint16:
		case uint32:
			roomId = uint64(val)
		case uint64:
			roomId = val
		default:
			return fmt.Errorf("expecting code `%s` to return with a room Id (type integer), bot got: %v", *room.code, val)
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
			room.OnEmit(room, ev.Event, ev.Args)
		}
	}
}
