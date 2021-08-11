# Go connector for ThingsDB

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
    * [Run(scope, procedure, args) -> interface{}, error](#Run)
    * [Emit(scope, roomId, event, args) -> interface{}, error](#Emit-Conn)
    * [Close()](#Close)
  * [Room](#Room)
    * [NewRoom(scope, code) -> *Room](#NewRoom)
    * [NewRoomFromId(scope, roomId) -> *Room](#NewRoomFromId)
    * [Id() -> uint64](#Id)
    * [Scope() -> string](#Scope)
    * [HandleEvent(event, handle)](#HandleEvent)
    * [Join(conn, wait) -> error](#Join)
    * [Leave() -> error](#Leave)
    * [Emit(event, args) -> interface{}, error](#Emit)

---------------------------------------

## Installation

Simple install the package to your [$GOPATH](https://github.com/golang/go/wiki/GOPATH) with the [go tool](https://golang.org/cmd/go/) from shell:

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

	// Arguments are optional, if no arguments are required, args may be `nil`
	args := map[string]interface{}{
		"a": 6,
		"b": 7,
	}

	// Just some example query using collection `stuff`
	if res, err := conn.Query("//stuff", "a * b;", args); err == nil {
		fmt.Println(res) // Should print 42, as 6 * 7 = 42
	} else {
		fmt.Println(err)
	}
}

func main() {
	// Optionally, the `NewConn` function accepts SSL configuration which might
	// be something like:
	//   conf := &tls.Config{
	// 	   InsecureSkipVerify: true,
	//   }
	conn := thingsdb.NewConn("localhost", 9200, nil)

	// With conn.AddNode(..) it is possible to add more than one node.
	// This will be used when a re-connect is triggerd to ensure that the
	// client quickly finds a new node to connect too. Adding another node is
	// not required when using a single ThingsDB node or when using a thingsdb
	// service which handles node distribution.

	ok := make(chan bool)
	go example(conn, ok)
	<-ok // Just to make sure the example code is completed before exit
}
```

## Conn

Type `Conn` is used as the ThingsDB Connection Client.

There are a few public properties which may be used:

Key            | Type          | Default      | Description
-------------- | ------------- | ------------ | -----------
DefaultTimeout | time.Duration | `0`          | Default time-out used when querying ThingsDB. When `0`, no time-out is used.
AutoReconnect  | bool          | `true`       | When `true`, the connection will try to re-connect when a connection is lost.
PingInterval   | time.Duration | `30s`        | Keep-alive ping interval. When `0`, keep-alive will be disabled.
LogCh          | chan string   | `nil`        | Forward logging to this channel. When `nil`, log will be send to `log.Println(..)`.
LogLevel       | LogLevelType  | `LogWarning` | Log level. Available levels: `LogDebug`, `LogInfo`, `LogWarning` and `LogError`.

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
    InsecureSkipVerify: true,
}
thingsdb.NewConn("localhost", 9200, config)
```

### AddNode

Add another node to the connection object. The connection will switch between
available nodes when a re-connect is triggered. This ensures that the connection
will be available very quickly after a ThingsDB node is restarted.

TLS configuration will be shared between all nodes. Thus, it is not possible to
enable TLS config (or a different) for a single node.

> Node: There is no point in adding another node when using only a single ThingsDB
> node, or when using a single thingsdb service, for example in Kubernetes

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

Create the ThingsDB connection.

### AuthPassword

Authenticate with a username and password.

### AuthToken

Authenticate with a token

### IsConnected

Returns `true` is the connector is connected to ThingsDB, `false` if not.

> Note: this function does not care is the connection is authenticated

### Query

Query ThingsDB using code.

*Example:*

```go
if res, err := conn.Query("/t", "'Hello Go Connector for ThingsDB!!';", nil); err == nil {
    fmt.Println(res)  // "Hello Go Connector for ThingsDB!!"
}
```

Arguments can be provided using a `map[string]interace{}`, for example:

```go
args := map[string]interface{}{
    "name": "Alice",
}

if res, err := conn.Query("/t", "`Hello {name}!!`;", args); err == nil {
    fmt.Println(res) // "Hello Alice!!"
}
```

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
args := map[string]interface{}{
    "a": 15,
    "b": 5,
}
if res, err := conn.Run("//stuff", "subtract", args); err == nil {
    fmt.Println(res)  // 10
}
```

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

Key        | Type         | Default      | Description
-----------| ------------ | ------------ | -----------
OnInit     | func(\*Room) | *dummy func* | Called only once at the first join. This function will run before the OnJoin.
OnJoin     | func(\*Room) | *dummy func* | Called at each join, thus also after a re-connect.
OnLeave    | func(\*Room) | *dummy func* | Called only when a room is explicitly left (A call to [room.Leave()](#Leave)).
OnDelete   | func(\*Room) | *dummy func* | Called when the room is removed from ThingsDB.
Data       | interface{}  | `nil`        | Can be used so assign additional data to the room.

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

Create a new room using code. The code should return the room Id for the room.

*Example:*

```go
// Suppose Collection stuff has a room (.room)
room := thingsdb.NewRoom("//stuff", ".room.id();")
```

### NewRoomFromId

Create a new room using a room Id.

If the room Id unknown, you may use [NewRoom](#NewRoom) to get the Id for the room by code.

*Example:*

```go
// Suppose Collection stuff has a room with Id 17
room := thingsdb.NewRoomFromId("//stuff", 17)
```

### Id

Return the Id for the room.

> Note: If the room was created using `NewRoom(..)`, then the Id will be `0` as long as the room is not joined.

### Scope

Return the scope of the room.

### HandleEvent

Add event handlers to the room.

*Example:*

```go
func onNewMessage(room *thingsdb.Room, args []interface{}) {
	if len(args) != 1 {
		fmt.Println("Invalid argument length")
		return
	}

	msg, ok := args[0].(string)
	if !ok {
		fmt.Println("Expecting message to type string")
		return
	}

	fmt.Println(msg)  // Just print the message
}

room = thingsdb.NewRoom("//stuff", ".chatRoom.id();")

// Add an event handler for the "new-message" event
room.HandleEvent("new-message", onNewMessage)
```

### Join

Function `Join()` must be called to actually join the room.

The `wait` argument may be set to `0` to tell the room not to wait for the join to complete.
If `wait` is set to any other positive value, then both the `OnInit` and `OnJoin` are called (in this order) before the call to Join returns unless the `OnJoin` is not completed before the `wait` duration (an error will be returned).

*Example:*

```go
err := room.Join(conn, thingsdb.DefaultWait)
```

### Leave

Use this function to stop listening for events on a room.

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