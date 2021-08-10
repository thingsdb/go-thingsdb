package main

import (
	"fmt"
	"time"

	"github.com/thingsdb/go-thingsdb"
)

func example(conn *thingsdb.Conn, ok chan bool) {
	var res interface{}
	var err error

	if err := conn.Connect(); err != nil {
		println(err.Error())
		ok <- false
		return
	}

	// oversight.Watch()

	defer conn.Close()

	// if err := conn.AuthToken("aoaOPzCZ1y+/f0S/jL1DUB"); err != nil {
	if err := conn.AuthPassword("admin", "pass"); err != nil {
		println(err.Error())
		ok <- false
		return
	}

	counter := 0

	for i := 0; i < 10; {
		time.Sleep(2 * time.Second)

		if res, err = conn.Query("@thingsdb", "collections_info();", nil, 120); err != nil {
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

	ok := make(chan bool)

	go example(conn, ok)

	<-ok
}
