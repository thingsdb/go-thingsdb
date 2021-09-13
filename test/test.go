package main

import (
	"fmt"
	"time"

	"github.com/thingsdb/go-thingsdb"
)

var stop bool

func onInit(room *thingsdb.Room) {
	fmt.Printf("In task room Id %d init\n", room.Id())
}

func onJoin(room *thingsdb.Room) {
	fmt.Printf("In task room Id %d join\n", room.Id())
}

func onMsg(room *thingsdb.Room, args []interface{}) {
	if len(args) != 1 {
		fmt.Println("Invalid argument length")
		return
	}

	msg, ok := args[0].(string)
	if !ok {
		fmt.Println("Expecting message to type string")
		return
	}

	if msg == "stop" {
		stop = true
	}

	fmt.Println(msg)
}

func example(conn *thingsdb.Conn, ok chan bool) {
	defer func(ok chan bool) { ok <- true }(ok)

	if err := conn.Connect(); err != nil {
		fmt.Println(err)
		return
	}

	defer conn.Close()

	if err := conn.AuthPassword("admin", "pass"); err != nil {
		fmt.Println(err)
		return
	}

	if _, err := conn.Query("//stuff", "!.has('room') && .room = room();", nil); err != nil {
		fmt.Println(err)
		return
	}

	room := thingsdb.NewRoom("//stuff", ".room.id();")
	room.OnInit = onInit
	room.OnJoin = onJoin
	room.HandleEvent("msg", onMsg)
	err := room.Join(conn, thingsdb.DefaultWait)
	if err != nil {
		fmt.Println(err)
	}

	counter := 0
	i := 0
	stop = false

	for i < 999 && !stop {
		time.Sleep(time.Second)

		if res, err := conn.Query("//stuff", ".keys();", nil); err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(res)
			counter += 1
		}
		i += 1
	}

	fmt.Printf("Succes count: %d  Total count: %d\n", counter, i)
}

func onNodeStatus(ns *thingsdb.NodeStatus) {
	println(ns.Status)
}

func onWarn(we *thingsdb.WarnEvent) {
	println(we.Msg)
}

func main() {
	// conf := &tls.Config{
	// 	InsecureSkipVerify: false,
	// }
	// conn := client.NewConn("35.204.223.30", 9400, conf)
	// conf := &tls.Config{InsecureSkipVerify: false}

	conn := thingsdb.NewConn("localhost", 9200, nil)

	conn.AddNode("localhost", 9201)
	conn.AddNode("localhost", 9202)

	conn.LogLevel = thingsdb.LogDebug

	// Optionally, we can set the OnNodeStatus and OnWarning callbacks...
	conn.OnNodeStatus = onNodeStatus
	conn.OnWarning = onWarn

	ok := make(chan bool)

	go example(conn, ok)

	<-ok
}
