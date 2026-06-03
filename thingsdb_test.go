package thingsdb

import (
	"crypto/tls"
	"os"
	"testing"
	"time"
)

// TestNewConn
func TestNewConn(t *testing.T) {
	conn := NewConn("loclhost", 9000, nil)

	if conn == nil {
		t.Fatalf("Failed to create a new connection")
	}
}

// TestPlayground
func TestPlayground(t *testing.T) {
	token := os.Getenv("TI_TOKEN")
	if token == "" {
		return
	}
	want := "Welcome at ThingsDB!"

	// Only required for a secure connection
	conf := &tls.Config{
		InsecureSkipVerify: false,
	}

	// Create a new ThingsDB connection
	conn := NewConn("playground.thingsdb.net", 9400, conf)

	if err := conn.Connect(); err != nil {
		t.Fatalf(`Failed to connect: %v`, err)
	} else {
		// Close the connection at the end of this function
		defer conn.Close()

		if err := conn.AuthToken(token); err != nil {
			t.Fatalf(`Failed to authenticate: %v`, err)
		} else {
			vars := map[string]any{
				"index": 1,
			}

			data, err := conn.Query(
				"//Doc",              // Scope
				".greetings[index];", // ThingsDB code
				vars,                 // Variable
			)

			if data != want || err != nil {
				t.Fatalf(`%q != %q, error: %v`, data, want, err)
			}
		}
	}
}

func TestLocal(t *testing.T) {
	do_local := os.Getenv("TI_LOCAL")
	if do_local == "" {
		return
	}
	conn := NewConn("127.0.0.1", 9200, nil)
	if err := conn.Connect(); err != nil {
		t.Fatalf(`Failed to connect: %v`, err)
	} else {
		// Close the connection at the end of this function
		defer conn.Close()
		if err := conn.AuthPassword("admin", "pass"); err != nil {
			t.Fatalf(`Failed to authenticate: %v`, err)
		} else {
			room := NewRoom("//stuff", "'my_room';")
			if err := room.Join(conn, time.Second*3); err != nil {
				t.Fatalf(`Failed to join: %v`, err)
			}
		}
	}
}
