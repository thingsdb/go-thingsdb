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
	var res interface{}
	var err error

	if err := conn.Connect(); err != nil {
		println(err.Error())
		ok <- false
		return
	}

	defer conn.Close()

	if err := conn.AuthPassword("admin", "pass"); err != nil {
		println(err.Error())
		ok <- false
		return
	}

	room := thingsdb.NewRoom("//stuff", ".room.id();")
	room.OnInit = onInit
	room.OnJoin = onJoin
	room.HandleEvent("msg", onMsg)
	err = room.Join(conn, thingsdb.DefaultTimeout)
	if err != nil {
		println(err.Error())
	}

	counter := 0
	stop = false

	for i := 0; i < 999 && !stop; {
		time.Sleep(2 * time.Second)

		if res, err = conn.Query("@thingsdb", "collections_info();", nil, thingsdb.DefaultTimeout); err != nil {
			println(err.Error())
		} else {
			fmt.Printf("%v\n", res)
			counter += 1
		}

		i += 1
	}

	fmt.Printf("Succes count: %d\n", counter)
	ok <- true
}

func main() {
	// conf := &tls.Config{
	// 	InsecureSkipVerify: true,
	// }
	// conn := client.NewConn("35.204.223.30", 9400, conf)
	conn := thingsdb.NewConn("localhost", 9200, nil)
	conn.LogLevel = thingsdb.LogDebug

	ok := make(chan bool)

	go example(conn, ok)

	<-ok
}
