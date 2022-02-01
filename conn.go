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
const maxReconnectSleep = time.Minute
const defaultReconnectionAttempts = 0  // 0 = infinite reconnect attempts

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
	// Private
	nodes    []node
	nodeIdx  int
	pid      uint16
	token    *string
	username *string
	password *string
	buf      *buffer
	respMap  map[uint16]chan *pkg
	config   *tls.Config
	mux      sync.Mutex
	rooms    *roomStore

	// Public
	DefaultTimeout       time.Duration
	AutoReconnect        bool
	ReconnectionAttempts int
	PingInterval         time.Duration
	LogCh                chan string
	LogLevel             LogLevelType
	OnNodeStatus         func(ns *NodeStatus)
	OnWarning            func(we *WarnEvent)
}

// NewConn creates a new ThingsDB Connector.
//
// Example:
//
//     thingsdb.NewConn("localhost", 9200, nil)
//
// Or, with TLS (SSL) config enabled
//
//     config := &tls.Config{
//         InsecureSkipVerify: false,
//     }
//     thingsdb.NewConn("localhost", 9200, config)
//
func NewConn(host string, port uint16, config *tls.Config) *Conn {
	return &Conn{
		// Private
		nodes:    []node{{host: host, port: port}},
		nodeIdx:  0,
		pid:      0,
		token:    nil,
		username: nil,
		password: nil,
		buf:      newBuffer(),
		respMap:  make(map[uint16]chan *pkg),
		config:   config,
		mux:      sync.Mutex{},
		rooms:    newRoomStore(),

		// Public
		DefaultTimeout:       0,
		AutoReconnect:        true,
		ReconnectionAttempts: defaultReconnectionAttempts,
		PingInterval:         defaultPingInterval,
		LogCh:                nil,
		LogLevel:             LogWarning,
		OnNodeStatus:         nil,
		OnWarning:            nil,
	}
}

// AddNone adds another node to the connector.

// The connection will switch between
// available nodes when a re-connect is triggered. This ensures that the connection
// will be available very quickly after a ThingsDB node is restarted.
//
// TLS configuration will be shared between all nodes. Thus, it is not possible to
// enable TLS config (or a different) for a single node.
//
// > Note: It is useless to add another node when using only a single ThingsDB
// node, or when using a single thingsdb service, for example in Kubernetes.
//
// Example:
//
//     conn := thingsdb.NewConn("node1.local", 9200, nil)
//     conn.AddNode("node2.local", 9200)
//     conn.AddNode("node3.local", 9200)
//
func (conn *Conn) AddNode(host string, port uint16) {
	conn.nodes = append(conn.nodes, node{host: host, port: port})
}

// ToString prints the current node address used by the connection.
//
// Example:
//
//     thingsdb.NewConn("localhost", 9200, nil).ToString()  // "localhost:9200"
//
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
		conn.nodeIdx = -1 // forces to start using node Id 0
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

// IsConnected returns `true` when the connector is connected to ThingsDB, `false` if not.
//
// > Note: this function does not care is the connection is authenticated
func (conn *Conn) IsConnected() bool {
	conn.mux.Lock()
	connected := conn.buf.conn != nil
	conn.mux.Unlock()

	return connected
}

// Query ThingsDB using code.
//
// Example:
//
//     if res, err := conn.Query("/t", "'Hello Go Connector for ThingsDB!!';", nil); err == nil {
//         fmt.Println(res)  // "Hello Go Connector for ThingsDB!!"
//     }
//
// Arguments can be provided using a `map[string]interface{}`, for example:
//
//     vars := map[string]interface{}{
//         "name": "Alice",
//     }
//
//     if res, err := conn.Query("/t", "`Hello {name}!!`;", vars); err == nil {
//         fmt.Println(res) // "Hello Alice!!"
//     }
//
func (conn *Conn) Query(scope string, code string, vars map[string]interface{}) (interface{}, error) {
	data := []interface{}{scope, code}
	if vars != nil {
		data = append(data, vars)
	}

	return conn.ensureWrite(ProtoReqQuery, data)
}

// Run a procedure in ThingsDB. Arguments are optional and may be either positional `[]interface{}` or by map `map[string]interface{}`.
//
// Example without arguments:
//
//     // Suppose collection `stuff` has the following procedure:
//     // new_procedure('greet', || 'Hi');
//
//     if res, err := conn.Run("//stuff", "greet", nil); err == nil {
//         fmt.Println(res)  // "Hi"
//     }
//
// Example using positional arguments:
//
//     // Suppose collection `stuff` has the following procedure:
//     // new_procedure('subtract', |a, b| a - b);
//
//     args := []interface{}{40, 10}
//
//     if res, err := conn.Run("//stuff", "subtract", args); err == nil {
//         fmt.Println(res)  // 30
//     }
//
// Example using mapped arguments:
//
//     // Suppose collection `stuff` has the following procedure:
//     // new_procedure('subtract', |a, b| a - b);
//
//     args := map[string]interface{}{
//         "a": 15,
//         "b": 5,
//     }
//     if res, err := conn.Run("//stuff", "subtract", args); err == nil {
//         fmt.Println(res)  // 10
//     }
// ```
func (conn *Conn) Run(scope string, procedure string, args interface{}) (interface{}, error) {
	data := []interface{}{scope, procedure}
	if args != nil {
		data = append(data, args)
	}

	return conn.ensureWrite(ProtoReqRun, data)
}

// Emit an even to a room.
//
// If a `Room` is created for the given `roomId`, you probable want to
// use Emit(..) on the `Room` type.
//
// Example:
//
//     args := []interface{}{"This is a message"}
//
//     err := conn.Emit(
//         "//stuff",      // scope of the Room
//         123,            // Room Id
//         "new-message",  // Event to emit
//         args            // Arguments (may be nil)
//     );
//
func (conn *Conn) Emit(scope string, roomId uint64, event string, args []interface{}) error {
	data := []interface{}{scope, roomId, event}
	if args != nil {
		data = append(data, args...)
	}

	_, err := conn.ensureWrite(ProtoReqEmit, data)
	return err
}

// Close an open connection.
//
// > Warning: After calling Close(), the `conn.AutoReconnect` property will be
// set to `false`. Thus, if you later want to `Connect()` again, make
// sure to re-enable this property manually.
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
		_, err := conn.ensureWrite(ProtoReqAuth, *conn.token)
		return err
	}

	if conn.username != nil && conn.password != nil {
		_, err := conn.ensureWrite(ProtoReqAuth, []string{*conn.username, *conn.password})
		return err
	}

	return fmt.Errorf("No authentication method available")
}

func (conn *Conn) connect() error {
	node := conn.node()
	if conn.config == nil {
		cn, err := net.Dial("tcp", conn.ToString())
		if err != nil {
			return err
		}
		conn.logInfo("Connected to %s:%d", node.host, node.port)
		conn.buf.conn = cn
	} else {
		cn, err := tls.Dial("tcp", conn.ToString(), conn.config)
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
	res, err := conn.ensureWrite(proto, data)

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

func (conn *Conn) nextPid() uint16 {
	conn.mux.Lock()
	pid := conn.pid
	conn.pid++
	conn.mux.Unlock()
	return pid
}

func (conn *Conn) getRespCh(pid uint16, b []byte, timeout time.Duration) (interface{}, error) {
	var err error
	respCh := make(chan *pkg, 1)

	conn.mux.Lock()
	conn.respMap[pid] = respCh

	if conn.buf.conn != nil {
		_, err = conn.buf.conn.Write(b)
	}

	conn.mux.Unlock()

	if err != nil {
		return nil, err
	}

	timeoutCh := make(chan bool, 1)

	if timeout > 0 {
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

func (conn *Conn) ensureWrite(tp Proto, data interface{}) (interface{}, error) {

	pid := conn.nextPid()
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

	pid := conn.nextPid()
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
			conn.logInfo("Try to reconnect...")
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
					if conn.OnNodeStatus != nil {
						conn.OnNodeStatus(nodeStatus)
					}

					if nodeStatus.Status == "SHUTTING_DOWN" {
						conn.closeAndReconnect("Node %d is shutting down... (%s)", nodeStatus.Id, conn.ToString())
					} else {
						conn.logInfo("Node %d has a new status: %v", nodeStatus.Id, nodeStatus.Status)
					}
				}
			case ProtoOnWarn:
				warnEvent, err := newWarnEvent(pkg)
				if err == nil {
					if conn.OnWarning == nil {
						conn.logWarning("ThingsDB: %s (%d)", warnEvent.Msg, warnEvent.Code)
					} else {
						conn.OnWarning(warnEvent)
					}
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
	for i := 1; conn.ReconnectionAttempts == 0 || i <= conn.ReconnectionAttempts; i++ {
		conn.nodeIdx += 1
		conn.nodeIdx %= len(conn.nodes)

		if err := conn.connect(); err == nil {
			// Authenticate the connection
			if conn.auth() != nil {
				// Re-join all the rooms
				roomMap := conn.rooms.getRoomMap()
				for scope, roomIds := range roomMap {
					if err := conn.join(scope, roomIds); err != nil {
						conn.logWarning("Unable to re-join all rooms: %v", err)
					}
				}
			} else {
				conn.logError("Authentication has failed")
			}
			break
		}

		conn.logInfo("Attempt %d: try to reconnect in %d second(s)...", i, sleep/time.Second)

		time.Sleep(sleep)

		sleep *= 2

		if sleep > maxReconnectSleep {
			sleep = maxReconnectSleep
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
