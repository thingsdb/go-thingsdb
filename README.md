# Go connector for ThingsDB

---------------------------------------

  * [Installation](#installation)
  * [Quick usage](#quick-usage)
  * [Conn](#Conn)
    * [NewConn(host, port, config)](#NewConn)
    * [AddNode(host, port)](#AddNode)
    * [ToString()](#ToString)
  * [Room](#Room)
    * [NewRoom()](#NewRoom)
    * [NewRoomFromId()](#NewRoomFromId)

---------------------------------------

## Installation

Simple install the package to your [$GOPATH](https://github.com/golang/go/wiki/GOPATH) with the [go tool](https://golang.org/cmd/go/) from shell:

```shell
$ go get github.com/thingsdb/go-thingsdb
```

Make sure [Git](https://git-scm.com/downloads) is installed on your machine and in your system's PATH.

## Quick usage

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

Example:

```go
conn := thingsdb.NewConn("localhost", 9200, nil)

// Change the log level
conn.LogLevel = thingsdb.LogInfo
```

## NewConn

Call `NewConn` to create a new ThingsDB Connection.

Example:

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

## AddNode

Add another node to the connection object. The connection will switch between
available nodes when a re-connect is triggered. This ensures that the connection
will be available very quickly after a ThingsDB node is restarted.

TLS configuration will be shared between all nodes. Thus, it is not possible to
enable TLS config (or a different) for a single node.

> Node: There is no point in adding another node when using only a single ThingsDB
> node, or when using a single thingsdb service, for example in Kubernetes

Example:

```go
conn.AddNode("node2.local", 9200)
```

## ToString

Prints the current node address used by the connection.

Example:

```go
thingsdb.NewConn("localhost", 9200, nil).ToString()  // "localhost:9200"
```

##