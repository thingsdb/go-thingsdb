package thingsdb

import (
	"crypto/tls"
	"testing"
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

		if err := conn.AuthToken("Fai6NmH7QYxA6WLYPdtgcy"); err != nil {
			t.Fatalf(`Failed to authenticate: %v`, err)
		} else {
			vars := map[string]interface{}{
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
