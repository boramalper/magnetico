package mainline

import (
	"net"
	"strings"
	"testing"
)

func TestReadFromOnClosedConn(t *testing.T) {
	// Initialization
	laddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	if err != nil {
		t.Skipf("Skipping due to an error during initialization!")
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		t.Skipf("Skipping due to an error during initialization!")
	}

	buffer := make([]byte, 65536)

	// Setting Up
	conn.Close()

	// Testing
	_, _, err = conn.ReadFrom(buffer)
	if !(err != nil && strings.HasSuffix(err.Error(), "use of closed network connection")) {
		t.Fatalf("Unexpected suffix in the error message!")
	}
}

func TestWriteToOnClosedConn(t *testing.T) {
	// Initialization
	laddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	if err != nil {
		t.Skipf("Skipping due to an error during initialization!")
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		t.Skipf("Skipping due to an error during initialization!")
	}

	// Setting Up
	conn.Close()

	// Testing
	_, err = conn.WriteTo([]byte("estarabim"), laddr)
	if !(err != nil && strings.HasSuffix(err.Error(), "use of closed network connection")) {
		t.Fatalf("Unexpected suffix in the error message!")
	}
}
