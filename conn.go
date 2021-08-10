package thingsdb

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

const defaultPingInterval = 30 * time.Second

// Conn is a ThingsDB connection to a single node.
type Conn struct {
	host          string
	port          uint16
	pid           uint16
	autoReconnect bool
	token         *string
	username      *string
	password      *string
	buf           *buffer
	respMap       map[uint16]chan *pkg
	ssl           *tls.Config
	mux           sync.Mutex
	rooms         *roomStore
	LogCh         chan string
	PingInterval  time.Duration
}

// NewConn creates a new connection
func NewConn(host string, port uint16, ssl *tls.Config) *Conn {
	return &Conn{
		host:          host,
		port:          port,
		pid:           0,
		autoReconnect: true,
		token:         nil,
		username:      nil,
		password:      nil,
		buf:           newBuffer(),
		respMap:       make(map[uint16]chan *pkg),
		rooms:         newRoomStore(),
		ssl:           ssl,
		LogCh:         nil,
		PingInterval:  defaultPingInterval,
	}
}

// ToString returns a string representing the connection and port.
func (conn *Conn) ToString() string {
	if strings.Count(conn.host, ":") > 0 {
		return fmt.Sprintf("[%s]:%d", conn.host, conn.port)
	}
	return fmt.Sprintf("%s:%d", conn.host, conn.port)
}

// Connect creates the TCP connection to the node.
func (conn *Conn) Connect() error {
	if conn.IsConnected() {
		return nil
	}

	if conn.ssl == nil {
		cn, err := net.Dial("tcp", conn.ToString())
		if err != nil {
			return err
		}
		conn.writeLog("connected to %s:%d", conn.host, conn.port)
		conn.buf.conn = cn
	} else {
		cn, err := tls.Dial("tcp", conn.ToString(), conn.ssl)
		if err != nil {
			return err
		}
		conn.writeLog("connected to %s:%d using a secure connection", conn.host, conn.port)
		conn.buf.conn = cn
	}

	go conn.buf.read()
	go conn.listen()
	if conn.PingInterval > 0 {
		go conn.ping()
	}

	return nil
}

// AuthPassword can be used to authenticate a connection using a username and
// password.
func (conn *Conn) AuthPassword(username, password string) error {
	_, err := conn.write(
		ProtoReqAuth,
		[]string{username, password},
		10)
	if err == nil {
		conn.username = &username
		conn.password = &password
	}
	return err
}

// AuthToken can be used to authenticate a connection using a token.
func (conn *Conn) AuthToken(token string) error {
	_, err := conn.write(
		ProtoReqAuth,
		token,
		10)
	if err == nil {
		conn.token = &token
	}
	return err
}

// IsConnected returns true when connected.
func (conn *Conn) IsConnected() bool {
	return conn.buf.conn != nil
}

// Query sends a query and returns the result.
func (conn *Conn) Query(scope string, query string, arguments map[string]interface{}, timeout uint16) (interface{}, error) {
	n := 3
	if arguments == nil {
		n = 2
	}
	data := make([]interface{}, n)
	data[0] = scope
	data[1] = query
	if arguments != nil {
		data[2] = arguments
	}

	return conn.write(ProtoReqQuery, data, timeout)
}

// Join room(s)
func (conn *Conn) Join(scope string, ids []uint64, timeout uint16) (interface{}, error) {
	data := make([]interface{}, 1+len(ids))
	data[0] = scope
	for i, v := range ids {
		data[1+i] = v
	}

	return conn.write(ProtoReqJoin, data, timeout)
}

// Leave room(s)
func (conn *Conn) Leave(scope string, ids []uint64, timeout uint16) (interface{}, error) {
	data := make([]interface{}, 1+len(ids))
	data[0] = scope
	for i, v := range ids {
		data[1+i] = v
	}

	return conn.write(ProtoReqLeave, data, timeout)
}

// Run can be used to run a stored procedure in a scope
// Note: `args` should be either an array with positional arguments or a map[string] with keyword arguments
func (conn *Conn) Run(procedure string, args interface{}, scope string, timeout uint16) (interface{}, error) {
	if len(procedure) == 0 {
		return nil, fmt.Errorf("No procedure given")
	}
	if len(scope) == 0 {
		return nil, fmt.Errorf("No scope given")
	}
	data := make([]interface{}, 3)
	data[0] = scope
	data[1] = procedure
	data[2] = args

	return conn.write(ProtoReqRun, data, timeout)
}

// Close will close an open connection.
func (conn *Conn) Close() {
	// Disable auto reconnect
	conn.autoReconnect = false

	if conn.buf.conn != nil {
		conn.writeLog("closing connection to %s:%d", conn.host, conn.port)
		conn.buf.conn.Close()
	}
}

func getResult(respCh chan *pkg, timeoutCh chan bool) (interface{}, error) {
	var result interface{}
	var err error

	select {
	case pkg := <-respCh:
		switch Proto(pkg.tp) {
		case ProtoResData:
			err = msgpack.Unmarshal(pkg.data, &result)
		case ProtoResPong, ProtoResOk:
			result = nil
		case ProtoResError:
			err = NewErrorFromByte(pkg.data)
		default:
			err = fmt.Errorf("unknown package type: %d", pkg.tp)
		}
	case <-timeoutCh:
		err = fmt.Errorf("query timeout reached")
	}

	return result, err
}

func (conn *Conn) increPid() uint16 {
	conn.mux.Lock()
	pid := conn.pid
	conn.pid++
	conn.mux.Unlock()
	return pid
}

func (conn *Conn) getRespCh(pid uint16, b []byte, timeout uint16) (interface{}, error) {
	respCh := make(chan *pkg, 1)

	conn.mux.Lock()
	conn.respMap[pid] = respCh
	conn.mux.Unlock()

	if conn.buf.conn != nil {
		conn.buf.conn.Write(b)
	}

	timeoutCh := make(chan bool, 1)

	if timeout != 0 {
		go func() {
			time.Sleep(time.Duration(timeout) * time.Second)
			timeoutCh <- true
		}()
	}

	result, err := getResult(respCh, timeoutCh)

	conn.mux.Lock()
	delete(conn.respMap, pid)
	conn.mux.Unlock()

	return result, err
}

func (conn *Conn) write(tp Proto, data interface{}, timeout uint16) (interface{}, error) {
	if !conn.IsConnected() {
		var i int
		for i = 0; i < int(timeout); i += 1 {
			if conn.IsConnected() {
				break
			}
			time.Sleep(1 * time.Second)
		}
		if !conn.IsConnected() {
			return nil, fmt.Errorf("not connected")
		}
	}
	pid := conn.increPid()
	b, err := pkgPack(pid, tp, data)

	if err != nil {
		return nil, err
	}

	return conn.getRespCh(pid, b, timeout)
}

func (conn *Conn) closeAndReconnect() {
	if conn.buf.conn != nil {
		conn.buf.conn.Close()
		conn.buf.conn = nil
		if conn.autoReconnect {
			go conn.reconnectLoop()
		}

	}
}

func (conn *Conn) listen() {
	for {
		select {
		case pkg := <-conn.buf.evCh:
			switch Proto(pkg.tp) {
			case ProtoOnNodeStatus:
				nodeStatus, err := newNodeStatus(pkg)
				if err == nil {
					if nodeStatus.Status == "SHUTTING_DOWN" {
						conn.writeLog("Node %d is shutting down... (%s:%d)", nodeStatus.Id, conn.host, conn.port)
						conn.closeAndReconnect()
					} else {
						conn.writeLog("Node %d has a new status: %v", nodeStatus.Id, nodeStatus.Status)
					}
				}
			case ProtoOnWarn:
				warnEvent, err := newWarnEvent(pkg)
				if err == nil {
					conn.writeLog("Warning from ThingsDB: %s (%d)", warnEvent.Msg, warnEvent.Code)
				}
			case ProtoOnRoomDelete, ProtoOnRoomEvent, ProtoOnRoomJoin, ProtoOnRoomLeave:

			}

		case pkg := <-conn.buf.pkgCh:
			conn.mux.Lock()
			if respCh, ok := conn.respMap[pkg.pid]; ok {
				conn.mux.Unlock()
				respCh <- pkg
			} else {
				conn.mux.Unlock()
				conn.writeLog("no response channel found for pid %d, probably the task has been cancelled ot timed out.", pkg.pid)
			}
		case err := <-conn.buf.errCh:
			conn.writeLog("%s (%s:%d)", niceErr(err), conn.host, conn.port)
			conn.closeAndReconnect()
		}
	}
}

func (conn *Conn) reconnectLoop() {
	sleep := 1 * time.Second

	for {
		conn.writeLog("attempt to reconnect to ThingsDB...")

		if err := conn.Connect(); err == nil {
			if conn.token != nil {
				conn.AuthToken(*conn.token)
			} else if conn.username != nil {
				conn.AuthPassword(*conn.username, *conn.password)
			}
			break
		}

		time.Sleep(sleep)

		if sleep < 120*time.Second {
			sleep *= 2
		}
	}
}

func (conn *Conn) writeLog(s string, a ...interface{}) {
	msg := fmt.Sprintf(s, a...)
	if conn.LogCh == nil {
		log.Println(msg)
	} else {
		conn.LogCh <- msg
	}
}

func (conn *Conn) ping() {
	for {
		time.Sleep(conn.PingInterval)
		if conn.IsConnected() {
			_, err := conn.write(ProtoReqPing, nil, 5)
			if err != nil {
				conn.writeLog("ping failed: %s", err)
			} else {
				conn.writeLog("ping! (%s:%d)", conn.host, conn.port)
			}
		} else {
			break
		}
	}
}
