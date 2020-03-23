package thingsdb

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"gopkg.in/vmihailenco/msgpack.v4"
)

const defaultPingInterval = 30

// Conn is a ThingsDB connection to a single node.
type Conn struct {
	host         string
	port         uint16
	pid          uint16
	buf          *buffer
	respMap      map[uint16]chan *pkg
	ssl          *tls.Config
	mux          sync.Mutex
	OnClose      func(error)
	EventCh      chan *Event
	LogCh        chan string
	PingInterval time.Duration
}

// NewConn creates a new connection
func NewConn(host string, port uint16, ssl *tls.Config) *Conn {
	return &Conn{
		host:         host,
		port:         port,
		pid:          0,
		buf:          newBuffer(),
		respMap:      make(map[uint16]chan *pkg),
		ssl:          ssl,
		OnClose:      nil,
		EventCh:      nil,
		LogCh:        nil,
		PingInterval: defaultPingInterval,
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

	return nil
}

// AuthPassword can be used to authenticate a connection using a username and
// password.
func (conn *Conn) AuthPassword(username, password string) error {
	_, err := conn.write(
		ProtoReqAuth,
		[]string{username, password},
		10)
	return err
}

// AuthToken can be used to authenticate a connection using a token.
func (conn *Conn) AuthToken(token string) error {
	_, err := conn.write(
		ProtoReqAuth,
		token,
		10)
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

// Subscribe for changes on given things.
func (conn *Conn) Watch(scope string, ids []uint64, timeout uint16) (interface{}, error) {
	data := make([]interface{}, 1)
	data[0] = scope
	for _, v := range ids {
		data = append(data, v)
	}
	return conn.write(ProtoReqWatch, data, timeout)
}

// Unsubscribe for changes on given things.
func (conn *Conn) Unwatch(scope string, ids []uint64, timeout uint16) (interface{}, error) {
	data := make([]interface{}, 1)
	data[0] = scope
	for _, v := range ids {
		data = append(data, v)
	}
	return conn.write(ProtoReqUnwatch, data, timeout)
}

/* Run procedure
Use this method to run a stored procedure in a scope.
Args:
	procedure (str):
		Name of the procedure to run.
	*args (any):
		Arguments which are injected as the procedure arguments.
		Can be of type map or array or be nil.
	scope (str, optional):
		Run the procedure in this scope.
		See https://docs.thingsdb.net/v0/overview/scopes/ for how to
		format a scope.
	timeout (int, optional):
		Raise a time-out exception if no response is received within X
		seconds. If no time-out is given, the client will wait forever.
*/
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

// Keep connection alive using the ping-pong protocol of ThingsDB
func (conn *Conn) EnableKeepAlive() {
	ok := make(chan bool)
	go conn.ping(ok)
	<-ok
}

// Close will close an open connection.
func (conn *Conn) Close() {
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
		case ProtoResPing, ProtoResOk:
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
		return nil, fmt.Errorf("not connected")
	}
	pid := conn.increPid()
	b, err := pkgPack(pid, tp, data)

	if err != nil {
		return nil, err
	}

	return conn.getRespCh(pid, b, timeout)
}

func (conn *Conn) listen() {
	for {
		select {
		case pkg := <-conn.buf.evCh:
			ev, err := newEvent(pkg)
			if err == nil {
				conn.EventCh <- ev
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
			conn.buf.conn.Close()
			conn.buf.conn = nil
			if conn.OnClose != nil {
				conn.OnClose(err)
			}
		}
	}
}

func (conn *Conn) writeLog(s string, a ...interface{}) {
	msg := fmt.Sprintf(s, a...)
	if conn.LogCh == nil {
		fmt.Println(msg)
	} else {
		conn.LogCh <- msg
	}
}

func (conn *Conn) ping(ok chan bool) {
	firstLoop := true
	continueLoop := true
	for continueLoop {
		if conn.IsConnected() {
			_, err := conn.write(ProtoReqPing, nil, 5)
			if err != nil {
				conn.writeLog("ping failed: %s", err)

			} else {
				conn.writeLog("ping! (%s:%d)", conn.host, conn.port)
			}
		} else {
			continueLoop = false
		}
		if firstLoop {
			firstLoop = false
			ok <- true
		}
		time.Sleep(conn.PingInterval * time.Second)
	}
}
