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
const pingTimout = 5 * time.Second
const authTimeout = 10 * time.Second

type LogLevelType int8

const (
	// Debug for debug logging
	LogDebug LogLevelType = 0
	// Info level
	LogInfo LogLevelType = 1
	// Warning level
	LogWarning LogLevelType = 2
	// Error level
	LogError LogLevelType = 3
)

// Conn is a ThingsDB connection to a single node.
type Conn struct {
	nodes          []node
	nodeIdx        int
	pid            uint16
	token          *string
	username       *string
	password       *string
	buf            *buffer
	respMap        map[uint16]chan *pkg
	ssl            *tls.Config
	mux            sync.Mutex
	rooms          *roomStore
	DefaultTimeout time.Duration
	AutoReconnect  bool
	PingInterval   time.Duration
	LogCh          chan string
	LogLevel       LogLevelType
}

// NewConn creates a new connection
func NewConn(host string, port uint16, ssl *tls.Config) *Conn {
	return &Conn{
		nodes:          []node{{host: host, port: port}},
		nodeIdx:        0,
		pid:            0,
		token:          nil,
		username:       nil,
		password:       nil,
		buf:            newBuffer(),
		respMap:        make(map[uint16]chan *pkg),
		ssl:            ssl,
		mux:            sync.Mutex{},
		rooms:          newRoomStore(),
		DefaultTimeout: 0,
		AutoReconnect:  true,
		PingInterval:   defaultPingInterval,
		LogCh:          nil,
		LogLevel:       LogWarning,
	}
}

func (conn *Conn) AddNode(host string, port uint16) {
	conn.nodes = append(conn.nodes, node{host: host, port: port})
}

// ToString returns a string representing the connection and port.
func (conn *Conn) ToString() string {
	node := conn.node()
	if strings.Count(node.host, ":") > 0 {
		return fmt.Sprintf("[%s]:%d", node.host, node.port)
	}
	return fmt.Sprintf("%s:%d", node.host, node.port)
}

// Connect creates the TCP connection to the node.
func (conn *Conn) Connect() error {
	if conn.IsConnected() {
		return nil
	}

	if conn.AutoReconnect {
		conn.reconnectLoop()
	} else {
		err := conn.connect()
		if err != nil {
			return err
		}
	}

	if conn.PingInterval > 0 {
		go conn.ping()
	}

	return nil
}

// AuthPassword can be used to authenticate a connection using a username and password.
func (conn *Conn) AuthPassword(username, password string) error {
	conn.username = &username
	conn.password = &password
	conn.token = nil
	_, err := conn.write(ProtoReqAuth, []string{*conn.username, *conn.password}, authTimeout)
	return err
}

// AuthToken can be used to authenticate a connection using a token.
func (conn *Conn) AuthToken(token string) error {
	conn.username = nil
	conn.password = nil
	conn.token = &token
	_, err := conn.write(ProtoReqAuth, *conn.token, authTimeout)
	return err
}

// IsConnected returns true when connected.
func (conn *Conn) IsConnected() bool {
	conn.mux.Lock()
	connected := conn.buf.conn != nil
	conn.mux.Unlock()

	return connected
}

// Query sends a query and returns the result.
func (conn *Conn) Query(scope string, code string, vars map[string]interface{}) (interface{}, error) {
	data := []interface{}{scope, code}
	if vars != nil {
		data = append(data, vars)
	}

	return conn.ensure_write(ProtoReqQuery, data)
}

// Run can be used to run a stored procedure in a scope
// Note: `args` should be an array with positional arguments or a map[string] with keyword arguments or nil
func (conn *Conn) Run(scope string, procedure string, args interface{}) (interface{}, error) {
	data := []interface{}{scope, procedure}
	if args != nil {
		data = append(data, args)
	}

	return conn.ensure_write(ProtoReqRun, data)
}

// Emit can be used to emit an event to a room
// Note: `args` should be an array with positional arguments or nil
func (conn *Conn) Emit(scope string, roomId uint64, event string, args []interface{}) (interface{}, error) {
	data := []interface{}{scope, roomId, event}
	if args != nil {
		data = append(data, args...)
	}

	return conn.ensure_write(ProtoReqRun, data)
}

// Close will close an open connection. This will also disable AutoReconnect
// so make sure to re-enable AutoReconnect if you wish to make a new call to
// Connect()
func (conn *Conn) Close() {
	conn.mux.Lock()

	// Disable auto reconnect
	conn.AutoReconnect = false

	if conn.buf.conn != nil {
		node := conn.node()
		conn.logWarning("Closing connection to %s:%d", node.host, node.port)
		conn.buf.conn.Close()
	}
	conn.mux.Unlock()
}

func (conn *Conn) node() *node {
	return &conn.nodes[conn.nodeIdx]
}

func (conn *Conn) auth() error {

	if conn.token != nil {
		_, err := conn.ensure_write(ProtoReqAuth, *conn.token)
		return err
	}

	if conn.username != nil && conn.password != nil {
		_, err := conn.ensure_write(ProtoReqAuth, []string{*conn.username, *conn.password})
		return err
	}

	return fmt.Errorf("No authentication method available")
}

func (conn *Conn) connect() error {
	node := conn.node()
	if conn.ssl == nil {
		cn, err := net.Dial("tcp", conn.ToString())
		if err != nil {
			return err
		}
		conn.logInfo("Connected to %s:%d", node.host, node.port)
		conn.buf.conn = cn
	} else {
		cn, err := tls.Dial("tcp", conn.ToString(), conn.ssl)
		if err != nil {
			return err
		}
		conn.logInfo("Connected to %s:%d using a secure connection", node.host, node.port)
		conn.buf.conn = cn
	}

	go conn.buf.read()
	go conn.listen()

	return nil
}

func (conn *Conn) joinOrLeave(proto Proto, scope string, ids []*uint64) error {
	data := make([]interface{}, 1+len(ids))
	data[0] = scope
	for i, v := range ids {
		data[1+i] = v
	}
	res, err := conn.ensure_write(ProtoReqJoin, data)
	if err == nil {
		arr, ok := res.([]interface{})
		if !ok {
			return fmt.Errorf("unexpected Join response: %v", res)
		}
		for i, val := range arr {
			switch val.(type) {
			case nil:
				ids[i] = nil
			}
		}
	}
	return err
}

func (conn *Conn) join(scope string, ids []*uint64) error {
	return conn.joinOrLeave(ProtoReqJoin, scope, ids)
}

func (conn *Conn) leave(scope string, ids []*uint64) error {
	return conn.joinOrLeave(ProtoReqLeave, scope, ids)
}

func getResult(respCh chan *pkg, timeoutCh chan bool) (interface{}, error) {
	var result interface{}
	var err error

	select {
	case pkg := <-respCh:
		if pkg == nil {
			err = NewTiError("request is cancelled", RequestCancelError)
		} else {
			switch Proto(pkg.tp) {
			case ProtoResData:
				err = msgpack.Unmarshal(pkg.data, &result)
			case ProtoResPong, ProtoResOk:
				result = nil
			case ProtoResError:
				err = NewTiErrorFromByte(pkg.data)
			default:
				err = fmt.Errorf("unknown package type: %d", pkg.tp)
			}
		}
	case <-timeoutCh:
		err = NewTiError("request timeout reached", RequestTimeoutError)
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

func (conn *Conn) getRespCh(pid uint16, b []byte, timeout time.Duration) (interface{}, error) {
	respCh := make(chan *pkg, 1)

	conn.mux.Lock()
	conn.respMap[pid] = respCh

	if conn.buf.conn != nil {
		conn.buf.conn.Write(b)
	}

	conn.mux.Unlock()

	timeoutCh := make(chan bool, 1)

	if timeout != 0 {
		go func() {
			time.Sleep(timeout)
			timeoutCh <- true
		}()
	}

	result, err := getResult(respCh, timeoutCh)

	conn.mux.Lock()
	delete(conn.respMap, pid)
	conn.mux.Unlock()

	return result, err
}

func (conn *Conn) ensure_write(tp Proto, data interface{}) (interface{}, error) {

	pid := conn.increPid()
	b, err := pkgPack(pid, tp, data)

	if err != nil {
		return nil, err
	}

	for ok := true; ok; ok = conn.AutoReconnect {
		if !conn.IsConnected() {
			conn.logInfo("Wait for a connection...")
			time.Sleep(time.Second)
			continue
		}

		res, err := conn.getRespCh(pid, b, conn.DefaultTimeout)
		if err == nil {
			return res, err
		}

		if terr, ok := err.(*TiError); ok {
			switch terr.Code() {
			case RequestTimeoutError, NodeError, RequestCancelError, AuthError:
				conn.logError("Err: %s (%d)", terr.Error(), terr.Code())
				time.Sleep(time.Second)
				continue
			}
		}

		return res, err
	}

	return nil, fmt.Errorf("not connected")

}

func (conn *Conn) write(tp Proto, data interface{}, timeout time.Duration) (interface{}, error) {
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

func (conn *Conn) closeAndReconnect(s string, a ...interface{}) {
	conn.mux.Lock()
	if conn.buf.conn != nil {
		conn.logWarning(s, a...)

		for _, channel := range conn.respMap {
			channel <- nil
		}

		prev := conn.buf.conn
		conn.buf.conn = nil
		prev.Close()

		if conn.AutoReconnect {
			go conn.reconnectLoop()
		}
	}
	conn.mux.Unlock()
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
						conn.closeAndReconnect("Node %d is shutting down... (%s)", nodeStatus.Id, conn.ToString())
					} else {
						conn.logInfo("Node %d has a new status: %v", nodeStatus.Id, nodeStatus.Status)
					}
				}
			case ProtoOnWarn:
				warnEvent, err := newWarnEvent(pkg)
				if err == nil {
					conn.logWarning("Warning from ThingsDB: %s (%d)", warnEvent.Msg, warnEvent.Code)
				}
			case ProtoOnRoomDelete, ProtoOnRoomEvent, ProtoOnRoomJoin, ProtoOnRoomLeave:
				ev, err := newRoomEvent(pkg)
				if err == nil {
					if room, ok := conn.rooms.getRoom(ev.Id); ok {
						room.onEvent(ev)
					} else {
						conn.logInfo("Room Id %d is not registered on this connection", ev.Id)
					}
				}
			}

		case pkg := <-conn.buf.pkgCh:
			conn.mux.Lock()
			if respCh, ok := conn.respMap[pkg.pid]; ok {
				conn.mux.Unlock()
				respCh <- pkg
			} else {
				conn.mux.Unlock()
				conn.logError("No response channel found for pid %d, probably the task has been cancelled ot timed out.", pkg.pid)
			}
		case err := <-conn.buf.errCh:
			conn.closeAndReconnect("Err: %s (%s)", niceErr(err), conn.ToString())
		}
	}
}

func (conn *Conn) reconnectLoop() {
	sleep := time.Second
	conn.logInfo("Try to reconnect...")
	for {
		conn.nodeIdx += 1
		conn.nodeIdx %= len(conn.nodes)

		if err := conn.connect(); err == nil {
			// Authenticate the connection
			conn.auth()

			// Re-join all the rooms
			roomMap := conn.rooms.getRoomMap()
			for scope, roomIds := range roomMap {
				conn.join(scope, roomIds)
			}
			break
		}

		conn.logInfo("Try to reconnect in %d second(s)...", sleep/time.Second)

		time.Sleep(sleep)

		if sleep < 120*time.Second {
			sleep *= 2
		}
	}
}

func (conn *Conn) _writeLog(s string, a ...interface{}) {
	msg := fmt.Sprintf(s, a...)
	if conn.LogCh == nil {
		log.Println(msg)
	} else {
		conn.LogCh <- msg
	}
}

func (conn *Conn) logDebug(s string, a ...interface{}) {
	if conn.LogLevel == LogDebug {
		conn._writeLog("[D] "+s, a...)
	}
}

func (conn *Conn) logInfo(s string, a ...interface{}) {
	if conn.LogLevel <= LogInfo {
		conn._writeLog("[I] "+s, a...)
	}
}

func (conn *Conn) logWarning(s string, a ...interface{}) {
	if conn.LogLevel <= LogWarning {
		conn._writeLog("[W] "+s, a...)
	}
}

func (conn *Conn) logError(s string, a ...interface{}) {
	if conn.LogLevel <= LogError {
		conn._writeLog("[E] "+s, a...)
	}
}

func (conn *Conn) ping() {
	for {
		time.Sleep(conn.PingInterval)

		if conn.IsConnected() {
			_, err := conn.write(ProtoReqPing, nil, pingTimout)
			if err != nil {
				conn.logError("Ping failed: %s", err)
			} else {
				conn.logInfo("Ping! (%s)", conn.ToString())
			}
		} else if !conn.AutoReconnect {
			break
		}
	}
}
