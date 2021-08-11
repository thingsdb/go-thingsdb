package thingsdb

// Proto is used as protocol type used by ThingsDB.
type Proto int8

const (

	/*
	 * Events
	 */

	// ProtoOnNodeStatus the connected node has changed it's status
	ProtoOnNodeStatus Proto = 0

	// ProtoOnWarn warning message for the connected client
	ProtoOnWarn Proto = 5

	// ProtoOnRoomJoin initial join
	ProtoOnRoomJoin Proto = 6

	// ProtoOnRoomLeave leave join
	ProtoOnRoomLeave Proto = 7

	// ProtoOnRoomEvent emit event
	ProtoOnRoomEvent Proto = 8

	// ProtoOnRoomDelete room removed from ThingsDB
	ProtoOnRoomDelete Proto = 9

	/*
	 * Responses
	 */

	// ProtoResPong responds with `nil`
	ProtoResPong Proto = 16
	// ProtoResOk responds with `nil`
	ProtoResOk Proto = 17
	// ProtoResData responds with `...`
	ProtoResData Proto = 18
	// ProtoResError responds with `{error_msg:..., error_code:,...}`
	ProtoResError Proto = 19

	/*
	 * Requests
	 */

	// ProtoReqPing requires `nil`
	ProtoReqPing Proto = 32
	// ProtoReqAuth requires `[username, password]`
	ProtoReqAuth Proto = 33
	// ProtoReqQuery requires `[scope, query [, variable]]`
	ProtoReqQuery Proto = 34
	// ProtoReqRun requires `[scope, procedure[, arguments]]`
	ProtoReqRun Proto = 37
	// ProtoReqJoin requires `[scope, room ids...]`
	ProtoReqJoin Proto = 38
	// ProtoReqLeave requires `[scope, room ids...]`
	ProtoReqLeave Proto = 39
	// ProtoReqEmit requires `[scope, roomId, event, arguments...]`
	ProtoReqEmit Proto = 40
)
