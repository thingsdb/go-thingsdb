[![CI](https://github.com/thingsdb/go-thingsdb/workflows/CI/badge.svg)](https://github.com/thingsdb/go-thingsdb/actions)
[![Release Version](https://img.shields.io/github/release/thingsdb/go-thingsdb)](https://github.com/thingsdb/go-thingsdb/releases)


# Go connector for ThingsDB

![Go ThingsDB](assets/go-thingsdb.png?raw=true "Go ThingsDB!!")

---------------------------------------

  * [Installation](#installation)
  * [Quick usage](#quick-usage)
  * [Conn](#Conn)
    * [NewConn(host, port, config) -> *Conn](#NewConn)
    * [AddNode(host, port)](#AddNode)
    * [ToString() -> string](#ToString)
    * [Connect() -> error](#Connect)
    * [AuthPassword(username, password) -> error](#AuthPassword)
    * [AuthToken(token) -> error](#AuthToken)
    * [IsConnected() -> bool](#IsConnected)
    * [Query(scope, code, vars) -> interface{}, error](#Query)
    * [QueryRaw(scope, code, vars) -> []byte, error](#QueryRaw)
    * [Run(scope, procedure, args) -> interface{}, error](#Run)
    * [RunRaw(scope, procedure, args) -> []byte, error](#RunRaw)
    * [Emit(scope, roomId, event, args) -> error](#Emit-Conn)
    * [Close()](#Close)
  * [Room](#Room)
    * [NewRoom(scope, code) -> *Room](#NewRoom)
    * [NewRoomFromId(scope, roomId) -> *Room](#NewRoomFromId)
	* [NewRoomFromName(scope, name) -> *Room](#NewRoomFromName)
    * [Id() -> uint64](#Id)
    * [Scope() -> string](#Scope)
    * [HandleEvent(event, handle)](#HandleEvent)
    * [Join(conn, wait) -> error](#Join)
    * [Leave() -> error](#Leave)
    * [Emit(event, args) -> error](#Emit)

---------------------------------------

## Installation

Simple install the package to your [$GOPATH](https://github.com/golang/go/wiki/GOPATH) with the [go tool](https://golang.org/cmd/go/) from shell:

> At least Go version 1.12 is required.

```shell
$ go get github.com/thingsdb/go-thingsdb
```

Make sure [Git](https://git-scm.com/downloads) is installed on your machine and in your system's PATH.

## Quick usage

This is a fully working example of how the Go ThingsDB connector can be used. It might seem a lot of code but this is mostly because comments are added to explain what each line does.

```go
package main

import (
	"fmt"

	"github.com/thingsdb/go-thingsdb"
)

func example(conn *thingsdb.Conn, ok chan bool) {
	defer func(ok chan bool) { ok <- true }(ok)

	// Create the connection, note that after calling connect you still need
	// too authenticate the connection.
	if err := conn.Connect(); err != nil {
		fmt.Println(err)
		return
	}

	// Make sure the connection will be closed at the end of this function
	defer conn.Close()

	// Here we use a username anf password to authenticate. It is also possible
	// to use a token with conn.AuthToken("my-secret-token").
	if err := conn.AuthPassword("admin", "pass"); err != nil {
		fmt.Println(err)
		return
	}

	// Arguments are optional, if no arguments are required, vars may be `nil`
	vars := map[string]interface{}{
		"a": 6,
		"b": 7,
	}

	// Just some example query using collection `stuff`
	if res, err := conn.Query("//stuff", "a * b;", vars); err == nil {
		fmt.Println(res) // Should print 42, as 6 * 7 = 42
	} else {
		fmt.Println(err)
	}
}

func main() {
	// Optionally, the `NewConn` function accepts TLS (SSL) configuration, for example:
	//
	//   config := &tls.Config{InsecureSkipVerify: false}
    //
	conn := thingsdb.NewConn("localhost", 9200, nil)

	// With conn.AddNode(..) it is possible to add more than one node.
	// This will be used when a re-connect is triggered to ensure that the
	// client quickly finds a new node to connect too. Adding another node is
	// not useful when using a single ThingsDB node or when using a thingsdb
	// service which handles node distribution.

	ok := make(chan bool)
	go example(conn, ok)
	<-ok // Just to make sure the example code is completed before exit
}
```

## Conn

Type `Conn` is used as the ThingsDB Connection Client.

There are a few public properties which may be used:

Key                  | Type               | Default      | Description
-------------------- | ------------------ | ------------ | -----------
DefaultTimeout       | time.Duration      | `0`          | Default time-out used when querying ThingsDB. When `0`, no time-out is used.
AutoReconnect        | bool               | `true`       | When `true`, the connection will try to re-connect when a connection is lost.
ReconnectionAttempts | int                | `0`          | Maximum number of re-connect attempts. When `0`, re-connect will try forever.
PingInterval   	     | time.Duration      | `30s`        | Keep-alive ping interval. When `0`, keep-alive will be disabled.
LogCh                | chan string        | `nil`        | Forward logging to this channel. When `nil`, log will be send to `log.Println(..)`.
LogLevel             | LogLevelType       | `LogWarning` | Log level. Available levels: `LogDebug`, `LogInfo`, `LogWarning` and `LogError`.
OnNodeStatus         | func(\*NodeStatus) | `nil`        | Called when a new node status is received. If implemented, the callback will be called before the client will handle the new status.
OnWarning            | func(\*WarnEvent)  | `nil`        | Called when a warning is received from ThingsDB. If *not* implemented (`nil`), the client will log a warning.

*Example:*

```go
conn := thingsdb.NewConn("localhost", 9200, nil)

// Change the log level
conn.LogLevel = thingsdb.LogInfo
```

### NewConn

Call `NewConn` to create a new ThingsDB Connection.

*Example:*

```go
thingsdb.NewConn("localhost", 9200, nil)
```

Or, with TLS (SSL) config enabled

```go
config := &tls.Config{
    InsecureSkipVerify: false,
}
thingsdb.NewConn("localhost", 9200, config)
```

### AddNode

Add another node to the connection object. The connection will switch between
available nodes when a re-connect is triggered. This ensures that the connection
will be available very quickly after a ThingsDB node is restarted.

TLS configuration will be shared between all nodes. Thus, it is not possible to
enable TLS config (or a different) for a single node.

> Note: It is useless to add another node when using only a single ThingsDB
> node, or when using a single thingsdb service, for example in Kubernetes.

*Example:*

```go
conn := thingsdb.NewConn("node1.local", 9200, nil)
conn.AddNode("node2.local", 9200)
conn.AddNode("node3.local", 9200)
```

### ToString

Prints the current node address used by the connection.

*Example:*

```go
thingsdb.NewConn("localhost", 9200, nil).ToString()  // "localhost:9200"
```

### Connect

Connect creates the TCP connection to the node.

### AuthPassword

AuthPassword can be used to authenticate a connection using a username and password.

### AuthToken

AuthToken can be used to authenticate a connection using a token.

### IsConnected

IsConnected returns `true` when the connector is connected to ThingsDB, `false` if not.

> Note: this function does not care is the connection is authenticated

### Query

Query ThingsDB using code.

*Example:*

```go
if res, err := conn.Query("/t", "'Hello Go Connector for ThingsDB!!';", nil); err == nil {
    fmt.Println(res)  // "Hello Go Connector for ThingsDB!!"
}
```

Arguments can be provided using a `map[string]interface{}`, for example:

```go
vars := map[string]interface{}{
    "name": "Alice",
}

if res, err := conn.Query("/t", "`Hello {name}!!`;", vars); err == nil {
    fmt.Println(res) // "Hello Alice!!"
}
```

### QueryRaw

The same as Query, except a raw `[]byte` array is returned.

### Run

Run a procedure in ThingsDB. Arguments are optional and may be either positional `[]interface{}` or by map `map[string]interface{}`.

*Example without arguments:*

```go
// Suppose collection `stuff` has the following procedure:
//
//   new_procedure('greet', || 'Hi');
//
if res, err := conn.Run("//stuff", "greet", nil); err == nil {
    fmt.Println(res)  // "Hi"
}
```

*Example using positional arguments:*

```go
// Suppose collection `stuff` has the following procedure:
//
//   new_procedure('subtract', |a, b| a - b);
//
args := []interface{}{40, 10}

if res, err := conn.Run("//stuff", "subtract", args); err == nil {
    fmt.Println(res)  // 30
}
```

*Example using mapped arguments:*

```go
// Suppose collection `stuff` has the following procedure:
//
//   new_procedure('subtract', |a, b| a - b);
//
vars := map[string]interface{}{
    "a": 15,
    "b": 5,
}
if res, err := conn.Run("//stuff", "subtract", vars); err == nil {
    fmt.Println(res)  // 10
}
```

### RunRaw

The same as Run, except a raw `[]byte` array is returned.

### Emit-Conn

Emit an even to a room. If a `Room` is created for the given `roomId`, you probable want to use [Emit](#Emit) on the [Room](#Room) type.

*Example:*

```go
args := []interface{}{"This is a message"}

err := conn.Emit(
    "//stuff",      // scope of the Room
    123,            // Room Id
    "new-message",  // Event to emit
    args            // Arguments (may be nil)
);
```

### Close

Close an open connection.

> Warning: After calling Close(), the `conn.AutoReconnect` property will be set to `false`. Thus, if you later want to `Connect()` again, make sure to re-enable this property manually.

## Room

Room type can be used to join a ThingsDB room.

Key        | Type         | Default      | Description
-----------| ------------ | ------------ | -----------
OnInit     | func(\*Room) | *dummy func* | Called only once at the first join. This function will run before the OnJoin.
OnJoin     | func(\*Room) | *dummy func* | Called at each join, thus also after a re-connect.
OnLeave    | func(\*Room) | *dummy func* | Called only when a room is explicitly left (A call to [room.Leave()](#Leave)).
OnDelete   | func(\*Room) | *dummy func* | Called when the room is removed from ThingsDB.
OnEmit     | func(\*Room, event *string*, args *[]interface{}*) | *dummy func* | Called only when [*no event handler*](#HandleEvent) is configured for the event.
Data       | interface{}  | `nil`        | Free to use, for example to assign additional data to the room *(Data stays untouched by the connector)*.

*Example configuring the OnInit and OnJoin functions:*

```go
func onInit(room *thingsdb.Room) {
	fmt.Printf("Initializing Room Id %d\n", room.Id())
    // Do some initialization... (called only once)
}

func onJoin(room *thingsdb.Room) {
	fmt.Printf("Joined Room Id %d\n", room.Id())
    // Do some stuff, maybe query ThingsDB... (called after each join, thus again after a re-connect)
}

// Suppose we have a chat room in collection stuff...
room := thingsdb.NewRoom("//stuff", ".chatRoom.id();")
room.OnInit = onInit
room.OnJoin = onJoin
```

### NewRoom

NewRoom creates a new room using code. The code should return the room Id for the room.

*Example:*

```go
// Suppose Collection stuff has a room (.room)
room := thingsdb.NewRoom("//stuff", ".room.id();")
```

### NewRoomFromId

NewRoomFromId creates a new room using a room Id.

If the room Id unknown, you may want use [NewRoom(..)](#NewRoom) to get the Id for the room by code.

*Example:*

```go
// Suppose Collection stuff has a room with Id 17
room := thingsdb.NewRoomFromId("//stuff", 17)
```


### NewRoomFromName

NewRoomFromName creates a new room using the name from a room.

Not every room has a name. The function [set_name]https://docs.thingsdb.io/v1/data-types/room/set_name/ must be used to give a room a name.
*Example:*

```go
// Suppose Collection stuff has a room with name "my_room"
room := thingsdb.NewRoomFromName("//stuff", "my_room")
```

### Id

Id returns the Id of the room.

> Note: Id() returns `0` when the room was created using [NewRoom(..)](#NewRoom) and the room has never been joined.

### Scope

Scope returns the Room Scope

### HandleEvent

HandleEvent adds an event handler to the room.

*Example:*

```go
func onNewMessage(room *thingsdb.Room, args []interface{}) {
	if len(args) != 1 {
		fmt.Println("Invalid number of arguments")
		return
	}

	msg, ok := args[0].(string)
	if !ok {
		fmt.Println("Expecting argument 1 to be of type string")
		return
	}

	fmt.Println(msg)  // Just print the message
}

room = thingsdb.NewRoom("//stuff", ".chatRoom.id();")

// Add event handler for the "new-message" event.
room.HandleEvent("new-message", onNewMessage)

// Note: The (optional) `OnEmit` handler will no longer be called for `new-message` events.
```

### Join

Join must be called to actually join the room.

The `wait` argument may be set to `0` to tell the room not to wait for the join to complete.
If `wait` is set to any other positive value, then both the `OnInit` and `OnJoin` are called (in this order) before the call to Join returns unless the `OnJoin` is not completed before the `wait` duration (an error will be returned).

*Example:*

```go
err := room.Join(conn, thingsdb.DefaultWait)
```

### Leave

Leave will stop listening for events on a room.

*Example:*

```go
err := room.Leave()
```

### Emit

Emit an even to a room.

*Example:*

```go
args := []interface{}{"Just some chat message"}

err := room.Emit(
    "new-message",  // Event to emit
    args            // Arguments (may be nil)
);
```
